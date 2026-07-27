package kubevirt

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/client-go/kubevirt/typed/core/v1"
)

const (
	// Time allowed to write a message to the peer.
	readWait  = 10 * time.Second
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	timeout = 5

	// Maximum message size allowed from peer.
	maxMessageSize  = 8192
	readBufferSize  = 1024
	WriteBufferSize = 1024

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// Simple function to check origin.
// origin header is always set by the browser.
// As it is, we authorize all origins since we're using Superphenix API anyway.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	fmt.Printf("Incoming connection from : %s\n", origin)
	return true
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: WriteBufferSize,
	CheckOrigin:     checkOrigin,
}

// TermConn represents the connected websocket and pty.
type TermConn struct {
	ws      *websocket.Conn
	con     net.Conn
	wsDone  chan struct{} // ws is closed, only close this chan in ws reader
	ptyDone chan struct{} // pty is closed, close this chan in pty reader
}

// Periodically send ping message to detect the status of the ws
func (tc *TermConn) ping(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

out:
	for {
		select {
		case <-ticker.C:
			err := tc.ws.WriteControl(websocket.PingMessage,
				[]byte{}, time.Now().Add(writeWait))

			if err != nil {
				log.Err(err).Msg("Failed to write ping message")
				break out
			}
		case <-tc.ptyDone:
			log.Info().Msg("Exit ping routine as pty is going away")
			break out

		case <-tc.wsDone:
			log.Info().Msg("Exit ping routine as ws is going away")
			break out
		}
	}

	log.Info().Msg("Ping routine exited")
}

// shovel data from websocket to pty stdin
func (tc *TermConn) wsToStdin(wg *sync.WaitGroup) {
	defer wg.Done()

	tc.ws.SetReadLimit(maxMessageSize)

	// set the readdeadline. The idea here is simple,
	// as long as we keep receiving pong message,
	// the readdeadline will keep updating. Otherwise
	// read will timeout.
	tc.ws.SetReadDeadline(time.Now().Add(pongWait))
	tc.ws.SetPongHandler(func(string) error {
		tc.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	bufChan := make(chan []byte)

	go func() { //create a goroutine to read from ws
		for {
			bufType, buf, err := tc.ws.ReadMessage()

			if err != nil {
				log.Err(err).Msg("Failed to receive data from ws")
				close(bufChan) // close chan by producer
				close(tc.wsDone)
				break
			}

			if bufType != websocket.BinaryMessage {
				bufChan <- buf
			}
		}
	}()
	// we do not need to forward user input to viewers, only the stdout
out:
	for {
		select {
		case buf, ok := <-bufChan:
			if !ok {
				log.Error().Msg("Exit wsToPtyStdin routine pty stdin error")
				break out
			}
			_, err := tc.con.Write(buf)

			if err != nil {
				log.Err(err).Msg("Failed to send data to pty stdin")
				break out
			}
		case <-tc.wsDone:
			log.Info().Msg("Exit wsToPtyStdin routine as ws is going away")
			break out
		case <-tc.ptyDone:
			log.Info().Msg("Exit wsToPtyStdin routine as pty is going away")
			break out
		}
	}

	log.Info().Msg("wsToPtyStdin routine exited")
}

// shovel data from pty Stdout to WS
func (tc *TermConn) stdoutToWs(wg *sync.WaitGroup) {
	defer wg.Done()
	bufChan := make(chan []byte)

	go func() { //create a goroutine to read from pty
		for {
			readBuf := make([]byte, 1024) //pty reads in 1024 blocks
			n, err := tc.con.Read(readBuf)

			if err != nil {
				log.Err(err).Msg("Failed to read from pty stdout")
				close(bufChan)
				close(tc.ptyDone)
				break
			}

			readBuf = readBuf[0:n] // slice the buffer so that it is exact the size of data read.
			bufChan <- readBuf
		}
	}()

out:
	for {
		// handle viewers, we want to use non-blocking receive
		select {
		case buf, ok := <-bufChan:
			if !ok {
				tc.ws.SetWriteDeadline(time.Now().Add(writeWait))
				tc.ws.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Pty closed"))

				break out
			}
			// We could add ws to viewers as well (then we can use io.MultiWriter),
			// but we want to handle errors differently
			tc.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := tc.ws.WriteMessage(websocket.BinaryMessage, buf); err != nil {
				log.Err(err).Msg("Failed to write message")
				break out
			}

		case <-tc.wsDone:
			log.Info().Msg("Exit ptyStdoutToWs routine as ws is going away")
			break out

		case <-tc.ptyDone:
			log.Info().Msg("Exit ptyStdoutToWs routine as pty is going away")
			break out // do not block on these two channels
		}

	}

	log.Info().Msg("ptyStdoutToWs routine exited")
}

// serialConsoleWS
//
//	@Summary		Websocket to Serial
//	@Description	Websocket to Serial Console of an instance
//	@Tags			v1, Instance
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		101
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/serial [get]
func serialConsoleWS(w http.ResponseWriter, r *http.Request, namespace, name string) {
	con, _ := config.VirtClient.VirtualMachineInstance(namespace).SerialConsole(name, &v1.SerialConsoleOptions{ConnectionTimeout: time.Duration(timeout) * time.Minute})

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Err(err).Msg("failed to upgrade connection")
		return
	}
	defer ws.Close()

	tc := TermConn{ws: ws}
	tc.con = con.AsConn()
	tc.wsDone = make(chan struct{})
	tc.ptyDone = make(chan struct{})

	// main event loop to shovel data between ws and pty
	// do not call ptyStdoutToWs in this goroutine, otherwise
	// the websocket will not close. This is because ptyStdoutToWs
	// is usually blocked in the pty.Read
	var wg sync.WaitGroup
	wg.Add(3)

	go tc.ping(&wg)
	go tc.stdoutToWs(&wg)
	go tc.wsToStdin(&wg)

	wg.Wait()

}

func SerialEndpoint(router chi.Router) {
	router.Get(baseVMEndpoint+"/{effectiveId}/serial", func(w http.ResponseWriter, r *http.Request) {
		l := logger.GetLogger(r.Context())
		namespace := utils.GetRequestNamespace(r)
		effectiveId := chi.URLParam(r, "effectiveId")
		if effectiveId == "" {
			l.Error().Ctx(r.Context()).Msg("no Resource Effective Id provided")
			httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
			return
		}

		_, err := config.VirtClient.VirtualMachineInstance(namespace).Get(r.Context(), effectiveId, k8smetav1.GetOptions{})
		if err != nil {
			l.Err(err).Msg("Failed to find the vmi")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
			return
		}
		l.Info().Msgf("Asking connection from : %s for [%s, %s]", r.RequestURI, namespace, effectiveId)
		serialConsoleWS(w, r, namespace, effectiveId)
	})
}

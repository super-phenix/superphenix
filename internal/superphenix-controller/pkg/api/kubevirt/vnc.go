package kubevirt

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Maximum message size allowed from peer.
	maxMessageSizeVNC  = 8192
	readBufferSizeVNC  = 1024
	WriteBufferSizeVNC = 1024
)

// Simple function to check origin.
// origin header is always set by the browser.
// As it is, we authorize all origins since we're using Superphenix API anyway.
func checkOriginVNC(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	fmt.Printf("Incoming connection from : %s\n", origin)
	return true
}

var upgraderVNC = websocket.Upgrader{
	ReadBufferSize:  readBufferSizeVNC,
	WriteBufferSize: WriteBufferSizeVNC,
	CheckOrigin:     checkOriginVNC,
}

// VNCWS
//
//	@Summary		Websocket to graphic VNC
//	@Description	Websocket to VNC of an instance
//	@Tags			v1, Instance
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		101
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/vnc [get]
func VNCWS(w http.ResponseWriter, r *http.Request, namespace, name string) {
	// preserve-session: This option will preserve an existing VNC session instead of dropping it.
	// default value : false
	// Testing with true in order to have permanent VNC session
	vncStream, err := config.VirtClient.VirtualMachineInstance(namespace).VNC(name, true)
	if err != nil {
		log.Error().Err(err).Msg("Get VNC preserved session failed")
		vncStream, err = config.VirtClient.VirtualMachineInstance(namespace).VNC(name, false)
		if err != nil {
			log.Error().Err(err).Msg("Get VNC failed")
			return
		}
	}

	// Upgrade WebSocket
	wsConn, err := upgraderVNC.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("webSocket upgrade failed")
		return
	}
	defer wsConn.Close()

	// VNC Connection
	vncConn := vncStream.AsConn()
	defer vncConn.Close()

	// Browser -> VNC
	go func() {
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || errors.Is(err, io.EOF) {
					log.Info().Msg("webSocket connection closed by client") // closed in an expected way
				} else {
					log.Error().Err(err).Msg("readMessage error") // closed in a unexpected way
				}
				return
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			_, err = vncConn.Write(data)
			if err != nil {
				log.Error().Err(err).Msg("write to VNC error")
				return
			}
		}
	}()

	// VNC -> Browser
	buf := make([]byte, 1024)
	for {
		n, err := vncConn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("read from VNC error")
			}
			return
		}
		err = wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			log.Error().Err(err).Msg("writeMessage error")
			return
		}
	}
}

func VNCEndpoint(router chi.Router) {
	router.Get(baseVMEndpoint+"/{effectiveId}/vnc", func(w http.ResponseWriter, r *http.Request) {
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
		VNCWS(w, r, namespace, effectiveId)
	})
}

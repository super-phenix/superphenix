from python:alpine

copy components/dependencies/talos-bootstrap/docker/pxe-setup/pxe-setup.py run.py

copy components/dependencies/talos-bootstrap/docker/pxe-setup/requirements.txt .

run pip install -r requirements.txt && pip cache purge

cmd python run.py
from python:alpine

copy components/dependencies/talos-bootstrap/docker/linkaliases-setup/linkaliases-setup.py run.py

copy components/dependencies/talos-bootstrap/docker/linkaliases-setup/requirements.txt .

run pip install -r requirements.txt && pip cache purge

cmd python run.py
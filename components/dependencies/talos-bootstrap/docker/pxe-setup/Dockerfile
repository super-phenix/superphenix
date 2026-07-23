FROM python:alpine

COPY components/dependencies/talos-bootstrap/docker/pxe-setup/pxe-setup.py run.py

COPY components/dependencies/talos-bootstrap/docker/pxe-setup/requirements.txt .

RUN pip install -r requirements.txt && pip cache purge

CMD python run.py
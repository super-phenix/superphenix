FROM python:alpine

COPY components/dependencies/talos-bootstrap/docker/linkaliases-setup/linkaliases-setup.py run.py

COPY components/dependencies/talos-bootstrap/docker/linkaliases-setup/requirements.txt .

RUN pip install -r requirements.txt && pip cache purge

CMD python run.py
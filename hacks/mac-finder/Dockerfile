FROM python:alpine

COPY mac-finder.py run.py

COPY requirements.txt .

RUN pip install -r requirements.txt && pip cache purge

CMD python run.py

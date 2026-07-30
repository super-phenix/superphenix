from python:alpine

copy mac-finder.py run.py

copy requirements.txt .

run pip install -r requirements.txt && pip cache purge

cmd python run.py

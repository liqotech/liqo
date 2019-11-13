FROM python:3

WORKDIR /usr/src/app

COPY ./drone .

RUN pip install --no-cache-dir -r requirements.txt

#CMD [ "python3", "./main.py", "cluster1", "-p", "-l", "DEBUG", "-d", "config/config.ini"]
# syntax=docker/dockerfile:1

FROM python:3.9-windowsservercore-1809

# Move to working directory /build
WORKDIR /build

RUN pip3 install prometheus-client

# Copy the code into the container
COPY . .

RUN dir

# Export necessary port
EXPOSE 2114

CMD [ "py", "app.py", "--host=0.0.0.0"]

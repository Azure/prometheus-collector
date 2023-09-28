# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/mirror/docker/library/python:3.8-slim

# Move to working directory /build
WORKDIR /build

RUN pip3 install prometheus-client

# Copy the code into the container
COPY . .

# Export necessary port
EXPOSE 2114

CMD [ "python3", "app.py", "--host=0.0.0.0"]

# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/windows-cssc/python:3.11-servercore-ltsc2022

# Move to working directory /build
WORKDIR /build

RUN python -m pip install prometheus-client

# Copy the code into the container
COPY . .

RUN dir

# Export necessary port
EXPOSE 2114

CMD [ "python", "app.py", "--host=0.0.0.0"]

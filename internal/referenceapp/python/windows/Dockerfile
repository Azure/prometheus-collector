# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/windows-cssc/python:3.11-servercore-ltsc2022

# Move to working directory /build
WORKDIR /build

RUN python -m pip install --upgrade pip setuptools wheel
RUN python -m pip --default-timeout=120 install --retries 10 prometheus-client
RUN python -m pip install --verbose prometheus-client

# Copy the code into the container
COPY . .

RUN dir

# Export necessary port
EXPOSE 2114

CMD [ "python", "app.py", "--host=0.0.0.0"]

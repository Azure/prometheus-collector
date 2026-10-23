# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/windows-cssc/python:3.11-servercore-ltsc2022

ARG PIP_INDEX_URL=https://pypi.org/simple

# Move to working directory /build
WORKDIR /build

COPY requirements.txt .
RUN python -m pip install --no-cache-dir --index-url "$env:PIP_INDEX_URL" -r requirements.txt

# Copy the code into the container
COPY . .

RUN dir

# Export necessary port
EXPOSE 2114

CMD [ "python", "app.py", "--host=0.0.0.0"]

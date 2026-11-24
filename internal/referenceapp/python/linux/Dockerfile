# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/azurelinux/base/python:3

ARG PIP_INDEX_URL=https://pypi.org/simple

# Move to working directory /build
WORKDIR /build

COPY requirements.txt .
RUN python3 -m pip install --no-cache-dir --index-url "$PIP_INDEX_URL" -r requirements.txt

# Copy the code into the container
COPY . .

# Export necessary port
EXPOSE 2114

CMD [ "python3", "app.py", "--host=0.0.0.0"]

# syntax=docker/dockerfile:1

FROM FROM mcr.microsoft.com/azurelinux/base/python:3

# Move to working directory /build
WORKDIR /build

RUN pip install prometheus-client

# Copy the code into the container
COPY . .

# Export necessary port
EXPOSE 2114

CMD [ "python", "app.py", "--host=0.0.0.0"]

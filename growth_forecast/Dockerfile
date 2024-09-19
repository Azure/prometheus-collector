FROM mcr.microsoft.com/cbl-mariner/base/python:3

# Copy files and install app dependencies.
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000

# Run forecast.py when the container launches
CMD ["python", "forecast.py"]
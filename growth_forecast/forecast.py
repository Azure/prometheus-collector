#!/usr/bin/env python3

"""
forecast.py

A script for forecasting growth using Azure Monitor data.
"""
import pandas as pd
import numpy as np
np.float_ = np.float64
from prophet import Prophet
from datetime import datetime, timedelta, timezone
import matplotlib
matplotlib.use("webagg")
from matplotlib import pyplot as plt
import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
import os, math
from azure.identity import DefaultAzureCredential
from azure.monitor.query import MetricsQueryClient, MetricAggregationType
pd.options.mode.chained_assignment = None
import prometheus_client
import prometheus_api_client, prometheus_api_client.utils
from apscheduler.schedulers.background import BackgroundScheduler
import logging
#logging.basicConfig(level=logging.DEBUG)
import json

_LOGGER = logging.getLogger(__name__)
# In case of a connection failure try 2 more times
MAX_REQUEST_RETRIES = 3
# wait 1 second before retrying in case of an error
RETRY_BACKOFF_FACTOR = 1
# retry only on these status
RETRY_ON_STATUS = [408, 429, 500, 502, 503, 504]
def _get_metric_range_data(
        self,
        metric_name: str,
        label_config: dict = None,
        start_time: datetime = (datetime.now() - timedelta(minutes=10)),
        end_time: datetime = datetime.now(),
        chunk_size: timedelta = None,
        store_locally: bool = False,
        params: dict = None,
):
    r"""
    Get the current metric value for the specified metric and label configuration.

    :param metric_name: (str) The name of the metric.
    :param label_config: (dict) A dictionary specifying metric labels and their
        values.
    :param start_time:  (datetime) A datetime object that specifies the metric range start time.
    :param end_time: (datetime) A datetime object that specifies the metric range end time.
    :param chunk_size: (timedelta) Duration of metric data downloaded in one request. For
        example, setting it to timedelta(hours=3) will download 3 hours worth of data in each
        request made to the prometheus host
    :param store_locally: (bool) If set to True, will store data locally at,
        `"./metrics/hostname/metric_date/name_time.json.bz2"`
    :param params: (dict) Optional dictionary containing GET parameters to be
        sent along with the API request, such as "time"
    :return: (list) A list of metric data for the specified metric in the given time
        range
    :raises:
        (RequestException) Raises an exception in case of a connection error
        (PrometheusApiClientException) Raises in case of non 200 response status code

    """
    params = params or {}
    data = []

    _LOGGER.debug("start_time: %s", start_time)
    _LOGGER.debug("end_time: %s", end_time)
    _LOGGER.debug("chunk_size: %s", chunk_size)

    if not (isinstance(start_time, datetime) and isinstance(end_time, datetime)):
        raise TypeError("start_time and end_time can only be of type datetime.datetime")

    if not chunk_size:
        chunk_size = end_time - start_time
    if not isinstance(chunk_size, timedelta):
        raise TypeError("chunk_size can only be of type datetime.timedelta")

    start = round(start_time.timestamp())
    end = round(end_time.timestamp())

    if end_time < start_time:
        raise ValueError("end_time must not be before start_time")

    if (end_time - start_time).total_seconds() < chunk_size.total_seconds():
        raise ValueError("specified chunk_size is too big")
    chunk_seconds = round(chunk_size.total_seconds())

    if label_config:
        label_list = [str(key + "=" + "'" + label_config[key] + "'") for key in label_config]
        query = metric_name + "{" + ",".join(label_list) + "}"
    else:
        query = metric_name
    #_LOGGER.debug("Prometheus Query: %s", query)

    while start < end:
        if start + chunk_seconds > end:
            chunk_seconds = end - start

        # using the query API to get raw data
        response = self._session.get(
            "{0}/api/v1/query_range".format(self.url),
            params={
                **{
                    "query": query,
                    "start": start,
                    "end": end,
                    "step": params["step"]
                },
                **params,
            },
            verify=self._session.verify,
            headers=self.headers,
            auth=self.auth,
            cert=self._session.cert
        )
        if response.status_code == 200:
            data += response.json()["data"]["result"]
        else:
            raise prometheus_api_client.PrometheusApiClientException(
                "HTTP Status Code {} ({!r})".format(response.status_code, response.content)
            )
        if store_locally:
            # store it locally
            self._store_metric_values_local(
                metric_name,
                json.dumps(response.json()["data"]["result"]),
                start + chunk_seconds,
            )

        start += chunk_seconds
    return data

prometheus_api_client.PrometheusConnect.get_metric_range_data = _get_metric_range_data

# Constants
SUBSCRIPTION_ID = "b9842c7c-1a38-4385-8f39-a51314758bcf"
RESOURCE_GROUP = "grace-addon"
RESOURCE_PROVIDER = "microsoft.monitor"
RESOURCE_TYPE = "accounts"
RESOURCE_NAME = "grace-addon"
METRIC_NAME = "EventsPerMinuteIngested"
TIMESPAN_HOURS = 24 * 45
GRANULARITY_HOURS = 1
AGGREGATION_TYPE = MetricAggregationType.MAXIMUM
TRAIN_SPLIT_RATIO = 0.99
FORECAST_PERIODS = 7 * 24
FORECAST_FREQUENCY = '1h'
LIMIT = 15000000
PROMETHEUS_PORT = 8000
METRICS_URI = f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP}/providers/{RESOURCE_PROVIDER}/{RESOURCE_TYPE}/{RESOURCE_NAME}"
PROMETHEUS_QUERY_URL = "https://grace-addon-mkvd.eastus.prometheus.monitor.azure.com"
PROMETHEUS_QUERY = 'sum(scrape_samples_post_metric_relabeling) by (job)'

class ProphetForecast:
    def __init__(self, train):
        self.train = train

    def fit_model(self, p, f, metric_name, limit):
        m = Prophet(daily_seasonality=True, weekly_seasonality=False, yearly_seasonality=False, interval_width=0.9)
        m.fit(self.train)
        future = m.make_future_dataframe(periods=p, freq=f)
        self.forecast = m.predict(future)

        fig = plt.figure(figsize=(20,10))
        plt.plot(np.array(self.train["ds"]), np.array(self.train["y"]),'b', label="train", linewidth=3)

        forecast_ds = np.array(self.forecast["ds"])
        plt.plot(forecast_ds, np.array(self.forecast["yhat"]), 'o', label="yhat", linewidth=3)
        plt.plot(forecast_ds, np.array(self.forecast["yhat_upper"]), 'y', label="yhat_upper", linewidth=3)
        plt.plot(forecast_ds, np.array(self.forecast["yhat_lower"]), 'y', label="yhat_lower", linewidth=3)
        plt.plot(forecast_ds, np.array(self.forecast["trend"]), 'g', label="trend", linewidth=2)
        #plt.axhline(y=limit, color='r', linestyle='-', label="limit")
        plt.xlabel("Timestamp")
        plt.ylabel("Value")
        plt.legend(loc=1)
        plt.title(metric_name)

        print(self.forecast[['trend']])

        return self.forecast
    
    def forecast_limit_reached(self):
        forecast_future = self.forecast.iloc[len(self.train['ds'])-1:,]
        forecast_future['threshold'] = forecast_future['yhat'].div(LIMIT).round(2).mul(100)
        forecast_future['threshold'] = forecast_future[forecast_future['threshold'] >= 100]['threshold']
        forecast_future = forecast_future.dropna()
        if forecast_future.empty:
            return None
        timestamp = forecast_future.iloc[0]['ds']
        return timestamp

def get_time_series_df(client, uri, metric_name, timespan_hours, granularity_hours, aggregation_type):
    response = client.query_resource(
        uri,
        metric_names=[metric_name],
        timespan=timedelta(hours=timespan_hours),
        granularity=timedelta(hours=granularity_hours),
        aggregations=[aggregation_type],
    )

    data = []
    for metric in response.metrics:
        print(metric.name + " -- " + metric.display_description)
        for time_series_element in metric.timeseries:
            for metric_value in time_series_element.data:
                row = []
                row.append(metric_value.timestamp.replace(tzinfo=None))
                row.append(metric_value.maximum)
                data.append(row)

    df = pd.DataFrame(data, columns=['ds', 'y'])
    df['ds'] = df['ds'].astype('datetime64[ns]')
    return df

def forecast_job(df, g, metric_name):
    pf = ProphetForecast(df)
    pf.fit_model(FORECAST_PERIODS, FORECAST_FREQUENCY, metric_name, LIMIT)
    timestamp = pf.forecast_limit_reached()
    print(timestamp)
    if timestamp is None:
        print("The limit will not be reached in the forecast period")
        g.set(-1)
        return
    difference = round((timestamp - datetime.now()) / timedelta(days=1))
    print("The limit will be reached at {} which is {} days from now".format(timestamp, difference))

    g.set(timestamp.timestamp())
    print(timestamp.timestamp())

def forecast_az_monitor_metrics(g):
    credential = DefaultAzureCredential()
    client = MetricsQueryClient(credential)
    df = get_time_series_df(client, METRICS_URI, METRIC_NAME, TIMESPAN_HOURS, GRANULARITY_HOURS, AGGREGATION_TYPE)
    forecast_job(df, g)

def forecast_prometheus_metrics(g):
    credential = DefaultAzureCredential()
    accessToken = credential.get_token("https://prometheus.monitor.azure.com")
    client = prometheus_api_client.PrometheusConnect(url=PROMETHEUS_QUERY_URL, headers={"Authorization": "Bearer {}".format(accessToken.token)})

    metric_data = client.get_metric_range_data(PROMETHEUS_QUERY, start_time=(datetime.now() - timedelta(days=14)), end_time=datetime.now(), params={"step": "1h"})
    print(metric_data)
    df = prometheus_api_client.MetricRangeDataFrame(metric_data)

    dataframeDict = {}
    job_names = df['job'].unique()
    for job in job_names:
        df_job = df[df['job'] == job]
        df_job = df_job.drop('job', axis=1)
        df_job = df_job.reset_index()
        df_job.columns = ['ds', 'y']
        print(df_job.head())
        print(df_job.info())
        dataframeDict[job] = df_job
        forecast_job(df_job, g, job)

def main():
    print("Welcome to the Growth Forecast Script")
    metric_limit_gauge = prometheus_client.Gauge('predicted_metric_limit_timestamp_seconds', 'Description of gauge')
    scrape_samples_gauge = prometheus_client.Gauge('predicted_scrape_samples_limit_timestamp_seconds', 'Description of gauge')
    #forecast_az_monitor_metrics(metric_limit_gauge)
    forecast_prometheus_metrics(scrape_samples_gauge)

    scheduler = BackgroundScheduler()
    #scheduler.add_job(lambda: forecast_az_monitor_metrics(metric_limit_gauge), 'interval', minutes=15)
    scheduler.add_job(lambda: forecast_prometheus_metrics(scrape_samples_gauge), 'interval', minutes=15)
    scheduler.start()
    plt.show()
    prometheus_client.start_http_server(PROMETHEUS_PORT)

    while True:
        pass

if __name__ == "__main__":
    main()

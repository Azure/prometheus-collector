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
from matplotlib import pyplot as plt
import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
import os, math
from azure.identity import DefaultAzureCredential
from azure.monitor.query import MetricsQueryClient, MetricAggregationType
pd.options.mode.chained_assignment = None
import prometheus_client
from apscheduler.schedulers.background import BackgroundScheduler

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
FORECAST_PERIODS = 30 * 24
FORECAST_FREQUENCY = '1h'
LIMIT = 15000000
PROMETHEUS_PORT = 8000
METRICS_URI = f"/subscriptions/{SUBSCRIPTION_ID}/resourceGroups/{RESOURCE_GROUP}/providers/{RESOURCE_PROVIDER}/{RESOURCE_TYPE}/{RESOURCE_NAME}"

class ProphetForecast:
    def __init__(self, train):
        self.train = train

    def fit_model(self, p, f, limit):
        m = Prophet(daily_seasonality=True, weekly_seasonality=True, yearly_seasonality=False, interval_width=0.9)
        m.fit(self.train)
        future = m.make_future_dataframe(periods=p, freq=f)
        self.forecast = m.predict(future)

        fig = plt.figure(figsize=(40,10))
        plt.plot(np.array(self.train["ds"]), np.array(self.train["y"]),'b', label="train", linewidth=3)

        forecast_ds = np.array(self.forecast["ds"])
        plt.plot(forecast_ds, np.array(self.forecast["yhat"]), 'o', label="yhat", linewidth=3)
        plt.plot(forecast_ds, np.array(self.forecast["yhat_upper"]), 'y', label="yhat_upper", linewidth=3)
        plt.plot(forecast_ds, np.array(self.forecast["yhat_lower"]), 'y', label="yhat_lower", linewidth=3)
        plt.axhline(y=limit, color='r', linestyle='-', label="limit")
        plt.xlabel("Timestamp")
        plt.ylabel("Value")
        plt.legend(loc=1)
        plt.title("Prophet Model Forecast")
        plt.show()

        return self.forecast
    
    def forecast_limit_reached(self):
        forecast_future = self.forecast.iloc[len(self.train['ds'])-1:,]
        forecast_future['threshold'] = forecast_future['yhat'].div(LIMIT).round(2).mul(100)
        forecast_future['threshold'] = forecast_future[forecast_future['threshold'] >= 100]['threshold']
        forecast_future = forecast_future.dropna()
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

def forecast_job(g):
    credential = DefaultAzureCredential()
    client = MetricsQueryClient(credential)
    df = get_time_series_df(client, METRICS_URI, METRIC_NAME, TIMESPAN_HOURS, GRANULARITY_HOURS, AGGREGATION_TYPE)

    pf = ProphetForecast(df)
    pf.fit_model(FORECAST_PERIODS, FORECAST_FREQUENCY, LIMIT)
    timestamp = pf.forecast_limit_reached()
    print(timestamp)
    difference = round((timestamp - datetime.now()) / timedelta(days=1))
    print("The limit will be reached at {} which is {} days from now".format(timestamp, difference))

    g.set(timestamp.timestamp())
    print(timestamp.timestamp())

def main():
    print("Welcome to the Growth Forecast Script")
    g = prometheus_client.Gauge('predicted_metric_limit_timestamp_seconds', 'Description of gauge')
    forecast_job(g)

    scheduler = BackgroundScheduler()
    scheduler.add_job(lambda: forecast_job(g), 'interval', minutes=15)
    scheduler.start()
    prometheus_client.start_http_server(PROMETHEUS_PORT)

    while True:
        pass

if __name__ == "__main__":
    main()

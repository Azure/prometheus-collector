# Limitations on metric Dimensions
There are some limitations on the dimensions of the metrics which is set by MDM. If any of the below limitations is exceeded, the entire batch is dropped in Grafana.

1. The  number of labels per timeseries should be < 64. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
2. The character length for label values should be < 1024. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
3. The character length for label names & metric names should be < 512. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
4. The labels per timeseries can be emoty as well.
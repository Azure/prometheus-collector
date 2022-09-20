RashmiTBD: Put this section in Grace's scale recommendations section

## Schedule ama-metrics replicaset pod on a nodepool with more resources
If the ama-metrics replicaset pod doesn't get scheduled on a node that has enough resources, it might keep getting OOMKilled and go to CrashLoopBackoff.
In order to overcome this, if you have a node on your cluster that has higher resources and want to get the replicaset scheduled on that node, you can 
add the label 'azuremonitor/metrics.replica.preferred=true' on the node and the replicaset pod will get scheduled on this node.

  ```
  kubectl label nodes <node-name> azuremonitor/metrics.replica.preferred=true
  ```



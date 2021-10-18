Following are some of the test cases to run through while making config processing changes

| Default config                            | Custom config | 
| -----------------------                   |:-------------:| 
| All default targets enabled               | Valid custom configmap    |
| All default targets enabled               | No custom configmap       |
| No default targets enabled                | Valid custom configmap    |
| No default targets enabled                | No custom configmap       |
| All or some default targets enabled       | Invalid custom configmap  |
| No default targets enabled                | Invalid custom configmap  |

**Test all of the above in simple and advanced mode**
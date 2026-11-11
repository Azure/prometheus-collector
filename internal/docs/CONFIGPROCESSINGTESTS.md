Following are some of the test cases to run through while making config processing changes

| Default config                            | Custom config | 
| -----------------------                   |:-------------:| 
| All default targets enabled               | Valid custom configmap    |
| All default targets enabled               | No custom configmap       |
| No default targets enabled                | Valid custom configmap    |
| No default targets enabled                | No custom configmap       |
| All or some default targets enabled       | Invalid custom configmap  |
| No default targets enabled                | Invalid custom configmap  |

The candidate settings catalog covers all six combinations above. The shipped
addon chart hard-codes `MODE` and `WINMODE` to `advanced` and documents it as
the only supported mode, so the historical simple-mode request is not applicable
to the current candidate deployment.
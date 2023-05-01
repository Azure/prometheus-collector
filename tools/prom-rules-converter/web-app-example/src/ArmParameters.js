import Form from "@rjsf/core";
import {useState} from 'react';

const schema = {
  title: "ARM options parameters",
  type: "object",
  properties: {
    azureMonitorWorkspace: {
      type: "string", 
      title: "Azure monitor workspace id"
    },
    clusterName: {
      type: "string",
      title: "Cluster name"
    },
    actionGroupId: {
      type: "string",
      title: "Action group Id"
    },
    location: {
      type: "string",
      title: "Action group location"
    },
    skipValidation: {
      type: "boolean",
      title: "Skip validation"
    }
  }
};

export default function ArmParametersForm(props) {
  const [formData, setFormData] = useState({});
  return <Form schema={schema}
              noHtml5Validate 
              showErrorList={false}
              formData={formData}
              onSubmit={({formData}) => { 
                            setFormData(formData);
                            props.onSubmit(formData);
                          }
              }/> 
} 


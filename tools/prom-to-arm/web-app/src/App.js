import 'bootstrap/dist/css/bootstrap.min.css';
import './App.css';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Stack from 'react-bootstrap/Stack';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form';

import Editor from "@monaco-editor/react";
import {useState, useEffect} from 'react';

import ArmParametersForm from './ArmParameters';
import toArmTemplate from 'azure-prom-tool';


function App() {
  const [yamlState, setYamlState] = useState('');
  const [optionsState, setOptionsState] = useState({});
  const [resultState, setResultState] = useState({success: true, output: ''});
  useEffect(
    () => {
      const res = toArmTemplate(yamlState, optionsState);
      setResultState(res);
    },
    [yamlState, optionsState],
  );
  return (
    <Container fluid>
    <Row>
      <Col xs={5}>
        <Stack gap={3}>
          <Card>
            <Card.Header>ARM Parameters Input</Card.Header>
            <Card.Body>
              <ArmParametersForm
                onSubmit={(formData) => setOptionsState(formData)}
              />
            </Card.Body>
          </Card>
          <Card>
            <Card.Header>
              <Form.Group controlId="formFile" className="mb-3">
                <Form.Label>Prometheus YAML input - choose a YAML file or type the YAML in the editor</Form.Label>
                <Form.Control type="file" onChange={e => showFile(e, setYamlState)}/>
              </Form.Group>
            </Card.Header>
            <Card.Body>
              <Editor
                height='100vh'
                defaultLanguage='yaml'
                value={yamlState}
                onChange={value => setYamlState(value)}
              />
            </Card.Body>
          </Card>
        </Stack>
        
      </Col>
      <Col xs={7}>
        {resultState.success ? (
          <Editor
            height='90vh'
            defaultLanguage='json'
            value={JSON.stringify(resultState, null, 2)}
            // onMount={console.log}
          />
        ) : (
          <div>
            <h1>{resultState?.error?.title}</h1>
            <Editor
              height='90vh'
              language='plaintext'
              value={JSON.stringify(resultState?.error?.details, null, 2)}
              // onMount={console.log}
            />
          </div>
        )}
      </Col>
    </Row>
    
    </Container>
  );
}

export default App;

const showFile = async (e, callBack) => {
  e.preventDefault();
  if (!e.target.files[0]) {
    callBack('');
    return;
  }
  const reader = new FileReader()
  reader.onload = async (e) => { 
    const text = (e.target.result);
    callBack(text);
  };
  reader.readAsText(e.target.files[0])
}
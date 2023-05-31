import 'bootstrap/dist/css/bootstrap.min.css';
import './App.css';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Stack from 'react-bootstrap/Stack';
import Card from 'react-bootstrap/Card';
import Form from 'react-bootstrap/Form';
import Button from 'react-bootstrap/Button';

import Editor from "@monaco-editor/react";
import {useState, useEffect} from 'react';

import ArmParametersForm from './ArmParameters';
import toArmTemplate from 'az-prom-rules-converter';


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
        <Card>
          <Card.Header>
            {resultState.success ? 
            (<>ARM template output <Button onClick={()=>saveFile(resultState.output)}>Save file</Button></>) : 
            (<><h1>ERROR:</h1>{resultState?.error?.title} </>)}
          </Card.Header>
          <Card.Body>
            <Editor
              height='90vh'
              defaultLanguage='json'
              value={JSON.stringify(resultState.success ? resultState.output : resultState?.error?.details, null, 2)}
            />
          </Card.Body>
        </Card>
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

const saveFile = (fileData, fileName = 'template.json') => {
  const blob = new Blob([JSON.stringify(fileData, null, 2)], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.download = fileName;
  link.href = url;
  link.click();
}
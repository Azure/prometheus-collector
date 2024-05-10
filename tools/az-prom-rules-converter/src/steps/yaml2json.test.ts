import yaml2json from './yaml2json'

test("Happy flow", () => {
  const json = yaml2json('bla: a');
  expect(json.success).toBe(true);
  expect(json.output).toEqual({bla: 'a'});
});




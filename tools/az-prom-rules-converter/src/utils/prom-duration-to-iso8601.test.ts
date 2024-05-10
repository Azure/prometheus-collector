import promDurationToISO8601 from './prom-duration-to-iso8601';
import moment from 'moment';

test("Success converting to ISO8601", () => {
  expect(promDurationToISO8601('2w')).toBe('P14D');
  expect(promDurationToISO8601('1ms')).toBe('PT0.001S');
  expect(promDurationToISO8601('1000ms')).toBe('PT1S');
  expect(promDurationToISO8601('1y2w3d4h5m6s')).toBe(moment.duration('P1Y2W3DT4H5M6S').toISOString());
});

test("Exception on bad format", () => {
  expect(()=>{promDurationToISO8601('bad format')}).toThrow(Error);
  expect(()=>{promDurationToISO8601('1ms2w')}).toThrow(Error);
  expect(()=>{promDurationToISO8601('PT1M')}).toThrow(Error);
});
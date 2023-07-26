import moment from 'moment'
import {unitOfTime} from 'moment'

const millisecond = 1,
second      = 1000 * millisecond,
minute      = 60   * second,
hour        = 60   * minute,
day         = 24   * hour,
week        = 7    * day,
year        = 356  * day;

const unitMap: Record<string, number> = {
  'ms': millisecond,
  's':  second,
  'm':  minute,
  'h':  hour,
  'd':  day,
  'w':  week,
  'y':  year
};

const durationRegex = /^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$/

function parseDuration(str: string) {
    const matches = str.match(durationRegex) || [];
    if (matches[0] != str) throw new Error(`${str} don't match ${durationRegex}`);
    const result: any = {};
    ['y','w','d','h','m','s','ms'].forEach((s,i) => {
      if (matches[(i+1)*2])
        result[s] = Number(matches[(i+1)*2]);
    });
    return result;
}

function toMomentDuration(parsedDuration: any) {
  let duration = moment.duration(0);
  for (const unit in parsedDuration) {
    duration = duration.add(parsedDuration[unit], (unit as unitOfTime.DurationConstructor));
  }
  return duration;
}

export default function promDurationToIso8601(promDuration: string) {
  const duration = parseDuration(promDuration);
  return toMomentDuration(duration).toISOString();
}
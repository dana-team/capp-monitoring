// load.js — sustained k6 load used to drive Knative autoscaling during the
// scale-behavior benchmark. The scale-test harness watches the autoscaler react
// while this runs; k6's own request metrics are secondary here.
import http from 'k6/http';

const VUS = __ENV.LOAD_VUS ? parseInt(__ENV.LOAD_VUS, 10) : 100;
const DURATION = __ENV.LOAD_DURATION || '3m';
const URL = (__ENV.TARGET_URL || '') + (__ENV.LOAD_PATH || '/');

export const options = {
  stages: [
    { duration: '30s', target: VUS }, // ramp up to push concurrency past the autoscaling target
    { duration: DURATION, target: VUS }, // hold to let pods scale out
  ],
};

export default function () {
  // No think-time: each VU stays continuously in-flight. Against a delay endpoint
  // (/delay/N) this holds ~VUs concurrent requests open — the signal the Knative
  // concurrency autoscaler scales on. With think-time the concurrency collapses.
  http.get(URL);
}

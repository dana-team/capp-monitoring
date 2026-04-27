import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 10 },
    { duration: '2m', target: 50 },
    { duration: '2m', target: 100 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://capp-backend.capp-system.svc.cluster.local:8080';

export default function () {
  const res = http.get(`${BASE_URL}/healthz`);
  check(res, { 'status is 200': (r) => r.status === 200 });
}

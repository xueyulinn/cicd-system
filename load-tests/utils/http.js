import http from "k6/http";

export function postJSON(baseURL, path, payload, endpoint) {
  const params = {
    headers: {
      "Content-Type": "application/json",
    },
    tags: { endpoint },
  };

  return http.post(`${baseURL}${path}`, JSON.stringify(payload), params);
}

export function parseJSON(response) {
  try {
    return response.json();
  } catch (_) {
    return null;
  }
}

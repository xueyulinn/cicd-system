import { check } from "k6";

export function checkValidateResponse(response, body) {
  return check(response, {
    "status is 200": (r) => r.status === 200,
    "response has valid(boolean)": () =>
      body !== null && typeof body.valid === "boolean",
    "validate result is valid=true": () => body !== null && body.valid === true,
  });
}

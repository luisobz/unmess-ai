// api.js — cliente HTTP autenticado contra el daemon local.
//
// El token de sesión vive aquí (no en el estado de UI): lo pide fetchToken() y
// lo guardan las peticiones. Si el daemon se reinició y regeneró el token (p. ej.
// tras una actualización o un `snap refresh`), la primera respuesta será 401: en
// ese caso refrescamos el token una vez y reintentamos, de modo que la sesión se
// recupera sola sin recargar la página.

let token = null;

// fetchToken pide un token nuevo al daemon y lo guarda.
export async function fetchToken() {
  const res = await fetch("/api/token");
  if (!res.ok) throw new Error("token");
  const j = await res.json();
  token = j.token;
  return token;
}

// refreshToken deduplica el refresco: si varias peticiones reciben 401 a la vez,
// todas esperan al mismo fetchToken en vuelo en lugar de disparar una estampida
// contra /api/token.
let refreshing = null;
function refreshToken() {
  if (!refreshing) {
    refreshing = fetchToken().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}

// api hace una petición autenticada, con un reintento tras refrescar el token si
// la respuesta es 401.
export async function api(path, opts = {}) {
  const send = () => {
    const headers = Object.assign({}, opts.headers || {});
    if (token) headers["Authorization"] = "Bearer " + token;
    return fetch(path, Object.assign({}, opts, { headers }));
  };
  let res = await send();
  if (res.status === 401) {
    try {
      await refreshToken();
    } catch (_) {
      return res; // el daemon no responde: devolvemos el 401 original
    }
    res = await send();
  }
  return res;
}

// apiJSON hace una petición autenticada y devuelve el JSON, lanzando un Error con
// el mensaje del servidor si la respuesta no es 2xx.
export async function apiJSON(path, opts = {}) {
  const res = await api(path, opts);
  if (!res.ok) {
    let msg = "Error " + res.status;
    try {
      const j = await res.json();
      if (j && j.error) msg = j.error;
    } catch (_) {}
    throw new Error(msg);
  }
  return res.json();
}

// jsonBody es un atajo para peticiones con cuerpo JSON.
export function jsonBody(method, data) {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  };
}

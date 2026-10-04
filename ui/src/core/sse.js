// sse.js — stream de eventos en vivo del daemon (/api/events).
//
// Usa fetch con lectura por streaming (no EventSource) para poder enviar el token
// en la cabecera Authorization. Reconecta solo si la conexión se cae.
import { api } from "./api.js";

// connectEvents abre el stream y entrega cada evento parseado a onEvent. Se
// reintenta indefinidamente con una espera fija entre intentos.
export async function connectEvents(onEvent) {
  try {
    const res = await api("/api/events", { headers: { Accept: "text/event-stream" } });
    if (!res.ok || !res.body) throw new Error("events " + res.status);
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      let idx;
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        for (const line of block.split("\n")) {
          const data = line.startsWith("data:") ? line.slice(5).trim() : "";
          if (!data) continue;
          try {
            onEvent(JSON.parse(data));
          } catch (_) {}
        }
      }
    }
  } catch (_) {
    // conexión caída o no soportada: se reintenta abajo
  }
  setTimeout(() => connectEvents(onEvent), 3000);
}

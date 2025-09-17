const express = require("express");
var Rcon = require("rcon");
const app = express();
var expressWs = require("express-ws")(app);
const port = 1337;
app.use(express.static("public"));

const getEnv = (name, replacement) => {
  const env = process.env[name];
  if (!env && !replacement) {
    throw new Error(`Missing env ${name}`);
  }
  return env ?? replacement;
};

const host = getEnv("RCON_HOST", "localhost");
const password = getEnv("RCON_PASSWORD");
const rcon_port = process.env.RCON_PORT ? Number(process.env.RCON_PORT) : 25575;
// Game type detection / configuration:
// Explicit via GAME_TYPE env, otherwise simple heuristic on port (25575 -> minecraft, else counter-strike)
const explicitGame = process.env.GAME_TYPE;
const gameType = explicitGame
  ? explicitGame.toLowerCase()
  : rcon_port === 25575
  ? "minecraft"
  : "counter-strike";
console.log(`[rcon2000] Starting for game type: ${gameType}`);

client = new Rcon(host, rcon_port, password, {
  tcp: true,
  challenge: false,
})
  .on("auth", () => {
    console.log("Authed!");
    client.send("help");
  })
  .on("response", (str) => {
    console.log("Got response: " + str);
    sendToAll(str?.length > 0 ? str : "Empty response");
  })
  .on("end", () => {
    console.log("Socket closed!");
  })
  .on("error", (err) => {
    console.log("Socket error: " + err);
    process.exit(1);
  });

const sendToAll = (msg) => {
  logListeners.forEach((ws) => {
    try {
      ws.send(msg);
    } catch (e) {
      console.error(e);
    }
  });
};

const logListeners = [];
const addSocketListener = (ws) => {
  logListeners.push(ws);
  ws.on("disconnect", () => {
    logListeners.splice(logListeners.indexOf(ws), 1);
  });
  ws.on("message", (msg) => {
    client.send(msg);
    sendToAll(msg);
  });
};

app.ws("/ws", function (ws, req) {
  addSocketListener(ws);
  // Send meta information first so UI can adapt
  try {
    ws.send(JSON.stringify({ type: "meta", game: gameType }));
  } catch (e) {
    console.error("Failed to send meta", e);
  }
  ws.send("Welcome to the multiplayer rcon!");
});

client.connect();

app.listen(port, () => {
  console.log(`Example app listening on port ${port}`);
});

const express = require("express");
var Rcon = require("rcon");
const app = express();
var expressWs = require("express-ws")(app);
const port = 1337;
app.use(express.static("public"));

const getEnv = (name) => {
  const env = process.env[name];
  if (!env) {
    throw new Error(`Missing env ${name}`);
  }
  return env;
};

const host = getEnv("RCON_HOST");
const password = getEnv("RCON_PASSWORD");
const rcon_port = process.env.RCON_PORT ? Number(process.env.RCON_PORT) : 27015;

client = new Rcon(host, rcon_port, password, {
  tcp: true, // false for UDP, true for TCP (default true)
  challenge: false, // true to use the challenge protocol (default true)
})
  .on("auth", () => {
    console.log("Authed!");
    client.send("help");
  })
  .on("response", (str) => {
    console.log("Got response: " + str);
    sendToAll(str);
  })
  .on("end", () => {
    console.log("Socket closed!");
  })
  .on("error", (err) => {
    console.log("Socket error: " + err);
  });

const sendToAll = (msg) => {
  logListeners.forEach((ws) => {
    ws.send(msg);
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
  });
};

app.ws("/ws", function (ws, req) {
  addSocketListener(ws);
  ws.send("Welcome to the multiplayer rcon!");
});

client.connect();

app.listen(port, () => {
  console.log(`Example app listening on port ${port}`);
});

(function (doc, w) {
  const $ = (id) => doc.getElementById(id);
  const messagesEl = $("messages");
  const commandInput = $("command");
  const form = $("form");
  const wsStatus = $("ws-status");
  const clearBtn = $("clear-log");
  const pauseBtn = $("pause-scroll");
  const toggleToolsBtn = $("toggle-tools");

  toggleToolsBtn.addEventListener("click", () => {
    const visible = doc.body.classList.toggle("tools-visible");
    toggleToolsBtn.setAttribute("aria-expanded", String(visible));
    toggleToolsBtn.textContent = visible ? "Hide" : "Tools";
  });

  // Ensure correct state when resizing from mobile to desktop
  function handleResize() {
    if (w.innerWidth > 1100) {
      // always show panels on desktop layout; remove attribute reliance
      toggleToolsBtn.setAttribute("aria-expanded", "true");
      doc.body.classList.add("tools-visible");
      toggleToolsBtn.textContent = "Tools";
    } else if (!doc.body.classList.contains("tools-visible")) {
      toggleToolsBtn.setAttribute("aria-expanded", "false");
      toggleToolsBtn.textContent = "Tools";
    }
  }
  w.addEventListener("resize", handleResize);
  handleResize();

  // Command history
  const HISTORY_KEY = "rcon2000.history";
  let history = [];
  try {
    history = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
  } catch (_) {
    history = [];
  }
  let historyIndex = history.length;
  const saveHistory = () =>
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history.slice(-200)));

  function htmlEncode(input) {
    return input
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  let autoScroll = true;
  pauseBtn.addEventListener("click", () => {
    autoScroll = !autoScroll;
    pauseBtn.textContent = autoScroll ? "Pause" : "Resume";
    messagesEl.classList.toggle("paused", !autoScroll);
    if (autoScroll) scrollToBottom(true);
  });
  clearBtn.addEventListener("click", () => {
    messagesEl.innerHTML = "";
  });

  function scrollToBottom(force = false) {
    if ((!autoScroll && !force) || messagesEl.scrollTop < 30) return;
    messagesEl.scrollTop = messagesEl.scrollHeight + 200;
  }
  let scrollDebounce;
  messagesEl.addEventListener("scroll", () => {
    clearTimeout(scrollDebounce);
    scrollDebounce = setTimeout(() => {
      const nearBottom =
        messagesEl.scrollHeight -
          messagesEl.scrollTop -
          messagesEl.clientHeight <
        40;
      if (!autoScroll && nearBottom) {
        autoScroll = true;
        pauseBtn.textContent = "Pause";
        messagesEl.classList.remove("paused");
      } else if (autoScroll && !nearBottom) {
        autoScroll = false;
        pauseBtn.textContent = "Resume";
        messagesEl.classList.add("paused");
      }
    }, 90);
  });

  function addMessage(raw, cls = "") {
    const msg = doc.createElement("div");
    msg.className = "msg " + cls;
    const span = doc.createElement("span");
    span.className = "payload";
    span.innerHTML = htmlEncode(raw);
    msg.appendChild(span);
    messagesEl.appendChild(msg);
    scrollToBottom();
  }

  // Dynamic game panel toggling
  function applyGame(g) {
    if (!g) return;
    const csPanel = doc.getElementById("csPanel");
    const mcPanel = doc.getElementById("mcPanel");
    if (csPanel && mcPanel) {
      if (g === "minecraft") {
        if (csPanel !== null) {
          csPanel.style.display = "none";
        }
        mcPanel.style.display = "flex";
        if (wsStatus && !mcPanel.querySelector("#ws-status")) {
          mcPanel.querySelector("h2").appendChild(wsStatus);
        }
      } else if (g === "counter-strike" || g === "cs" || g === "csgo") {
        csPanel.style.display = "flex";
        mcPanel.style.display = "none";
      } else {
        csPanel.style.display = "flex";
        mcPanel.style.display = "flex";
      }
    }
    addMessage(`Game context: ${g}`, "system");
  }

  // Reconnecting WebSocket with backoff and queue
  let socket = null,
    reconnectAttempts = 0,
    manualClose = false;
  const MAX_BACKOFF = 15000;
  const queued = [];
  let currentGame = null;
  function humanDelay(ms) {
    if (ms < 1000) return ms + "ms";
    return (ms / 1000).toFixed(1) + "s";
  }
  function updateStatus(state) {
    if (state === true || state === "online") {
      wsStatus.textContent = "ONLINE";
      wsStatus.classList.remove("off");
    } else if (state === "connecting") {
      wsStatus.textContent = "CONNECTING";
      wsStatus.classList.remove("off");
    } else {
      wsStatus.textContent = "OFFLINE";
      wsStatus.classList.add("off");
    }
  }
  function createSocket() {
    updateStatus("connecting");
    const url = `${location.protocol.replace("http", "ws")}//${
      location.host
    }/ws`;
    socket = new WebSocket(url);
    socket.addEventListener("open", () => {
      updateStatus(true);
      messagesEl.innerHTML = "";
      addMessage("Connected to server.", "system"); // flush queue
      while (queued.length) {
        const q = queued.shift();
        try {
          socket.send(q);
          //addMessage("> " + q, "system");
        } catch (e) {
          queued.unshift(q);
          break;
        }
      }
      reconnectAttempts = 0;
    });
    socket.addEventListener("close", () => {
      if (manualClose) return;
      updateStatus(false);
      const delay =
        Math.min(MAX_BACKOFF, 500 * Math.pow(2, reconnectAttempts++)) +
        Math.round(Math.random() * 250);
      addMessage(
        `Connection closed. Reconnecting in ${humanDelay(delay)} ...`,
        "error"
      );
      setTimeout(createSocket, delay);
    });
    socket.addEventListener("error", () => {
      updateStatus(false);
      addMessage("WebSocket error occurred.", "error");
    });
    socket.addEventListener("message", (event) => {
      try {
        if (event.data.startsWith("{")) {
          const obj = JSON.parse(event.data);
          if (obj && obj.type === "meta" && obj.game) {
            currentGame = obj.game;
            applyGame(obj.game);
            return;
          }
        }
      } catch (e) {}
      addMessage(event.data);
    });
  }
  createSocket();

  function sendCommand(cmd) {
    if (!cmd.trim()) return;
    if (socket && socket.readyState === 1) {
      try {
        socket.send(cmd);
      } catch (e) {
        queued.push(cmd);
      }
    } else {
      queued.push(cmd);
    }
    addMessage("> " + cmd, "system flash");
    history.push(cmd);
    historyIndex = history.length;
    saveHistory();
  }

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    sendCommand(commandInput.value);
    commandInput.value = "";
  });
  commandInput.addEventListener("keydown", (e) => {
    if (e.key === "ArrowUp") {
      if (historyIndex > 0) historyIndex--;
      commandInput.value = history[historyIndex] || "";
      setTimeout(
        () =>
          commandInput.setSelectionRange(
            commandInput.value.length,
            commandInput.value.length
          ),
        0
      );
      e.preventDefault();
    } else if (e.key === "ArrowDown") {
      if (historyIndex < history.length) historyIndex++;
      commandInput.value = history[historyIndex] || "";
      setTimeout(
        () =>
          commandInput.setSelectionRange(
            commandInput.value.length,
            commandInput.value.length
          ),
        0
      );
      e.preventDefault();
    }
  });

  // Quick command buttons
  doc.querySelectorAll("button[data-cmd]").forEach((btn) => {
    btn.addEventListener("click", () => {
      btn.dataset.cmd
        .trim()
        .split(/[\n\r\t]+/g).split(";").filter(a => a).forEach(cmd => {    
          sendCommand(cmd);
        });
        
    });
  });

  // Global keyboard focus
  doc.addEventListener("keydown", (e) => {
    if (e.key === "/" && doc.activeElement !== commandInput) {
      e.preventDefault();
      commandInput.focus();
    }
  });
  w._rconUI = { addMessage, sendCommand, applyGame };
})(document, window);

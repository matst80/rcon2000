document.addEventListener("DOMContentLoaded", () => {
  const logContainer = document.createElement("div");
  logContainer.id = "log-container";
  logContainer.style.display = "none";
  logContainer.innerHTML = '<h2>Game Server Logs</h2><div id="logs"></div>';
  document.body.appendChild(logContainer);

  const controls = document.createElement("button");
  controls.id = "gameServerBtn";
  controls.className = "k8s small";
  controls.disabled = true;
  controls.innerHTML = `Show logs`;
  controls.addEventListener(
    "click",
    () => {
      if (logContainer.style.display === "none") {
        logContainer.style.display = "block";
        controls.textContent = "Hide logs";
      } else {
        logContainer.style.display = "none";
        controls.textContent = "Show logs";
      }
    },
    false
  );
  document.querySelector("div.toolbar")?.appendChild(controls);

  const logsDiv = document.getElementById("logs");
  let socket;

  function connect() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    socket = new WebSocket(`${protocol}//${host}/api/logs`);

    socket.onopen = () => {
      console.log("Log socket connected");
      logsDiv.innerHTML = ""; // Clear previous logs
      controls.disabled = false;
    };

    socket.onmessage = (event) => {
      const logEntry = document.createElement("div");
      logEntry.textContent = event.data;
      logsDiv.appendChild(logEntry);
      logsDiv.scrollTop = logsDiv.scrollHeight; // Auto-scroll
    };

    socket.onclose = () => {
      console.log("Log socket disconnected, attempting to reconnect...");
      setTimeout(connect, 5000); // Reconnect after 5 seconds
      controls.disabled = true;
    };

    socket.onerror = (error) => {
      console.error("Log socket error:", error);
      socket.close();
    };
  }

  connect();
});

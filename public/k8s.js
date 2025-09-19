var replicas = 0;

function updateGameServerStatus() {
  fetch("/api/gameserver")
    .then((response) => response.json())
    .then((data) => {
      console.log("Game server status:", data);
      replicas = data.readyReplicas || 0;
      updateGameServerButton();
    })
    .catch((err) => console.error("Error fetching game server status:", err));
}

function updateGameServerButton() {
  const button = document.getElementById("gameServerBtn");
  if (!button) return;
  button.disabled = false;
  if (replicas > 0) {
    button.textContent = "Stop";
    button.onclick = stopGameServer;
    button.classList.remove("start");
    button.classList.add("stop");
  } else {
    button.textContent = "Start";
    button.onclick = startGameServer;
    button.classList.remove("stop");
    button.classList.add("start");
  }
}

function startGameServer() {
  fetch("/api/gameserver", { method: "POST" })
    .then((data) => {
      if (data.ok) {
        replicas = 1;
        updateGameServerButton();
        setTimeout(updateGameServerStatus, 5000); // Re-check after 5s
      }
    })
    .catch((err) => console.error("Error starting game server:", err));
}

function stopGameServer() {
  fetch("/api/gameserver", { method: "DELETE" })
    .then((response) => {
      if (response.ok) {
        replicas = 0;
        updateGameServerButton();
      }
    })
    .catch((err) => console.error("Error stopping game server:", err));
}

// Add button to the DOM
document.addEventListener("DOMContentLoaded", () => {
  const controls = document.createElement("button");
  controls.id = "gameServerBtn";
  controls.className = "k8s small";
  controls.innerHTML = `...`;
  controls.disabled = true;
  document.querySelector("div.toolbar")?.appendChild(controls);

  updateGameServerStatus();
});

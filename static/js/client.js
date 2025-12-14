(() => {
	// =========================================================
	//  ELEMENTS DU DOM
	// =========================================================
	const statusEl = document.getElementById("statusDot");
	const statusTextEl = document.getElementById("statusText");
	const connectButton = document.getElementById("connectButton");
	const urlInput = document.getElementById("urlInput");
	const copyButton = document.getElementById("copyButton");
	const copyIcon = document.getElementById("copyIcon");
	const checkIcon = document.getElementById("checkIcon");
	const copySuccess = document.getElementById("copySuccess");

	let ws = null;
	let manuallyDisconnected = false;

	// =========================================================
	//  COPIE DU LIEN (ancien code de main.js)
	// =========================================================
	copyButton.addEventListener("click", () => {
		const url = urlInput.value.trim();
		if (!url) return;

		navigator.clipboard.writeText(url).then(() => {
			// transition icônes
			copyIcon.style.display = "none";
			checkIcon.style.display = "block";

			setTimeout(() => {
				copySuccess.style.display = "none";
			}, 1500);

			setTimeout(() => {
				checkIcon.style.display = "none";
				copyIcon.style.display = "block";
			}, 2000);
		});
	});

	// =========================================================
	//  STATUS UI
	// =========================================================
	function updateStatus(connected) {
		statusEl.className =
			"status-dot " + (connected ? "connected" : "disconnected");
		statusTextEl.textContent = connected ? "Connecté" : "Déconnecté";

		connectButton.className =
			"connect-button " + (connected ? "connected" : "disconnected");
		connectButton.textContent = connected ? "Connecté" : "Déconnecté";
	}

	// =========================================================
	//  GESTION DE CONNEXION
	// =========================================================
	function connect() {
		ws = new WebSocket("ws://localhost:8080/ws");

		ws.onopen = () => {
			updateStatus(true);
			manuallyDisconnected = false;
		};

		ws.onmessage = (event) => {
			console.log("Message reçu :", event.data);
		};

		ws.onerror = () => {
			console.error("WebSocket error");
		};

		ws.onclose = () => {
			updateStatus(false);

			if (!manuallyDisconnected) {
				setTimeout(connect, 1000);
			}
		};
	}

	// Lancer connexion automatique
	connect();
})();

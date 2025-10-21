(() => {
	const statusEl = document.getElementById('status');
	const paragraph = document.getElementById('live-text');

	function connect() {
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
		const url = `${proto}://${location.host}/ws`;
		const ws = new WebSocket(url);

		ws.onopen = () => {
			statusEl.textContent = 'Connecté';
			statusEl.className = 'status online';
		};

		ws.onmessage = (evt) => {
			try {
				const data = JSON.parse(evt.data);
				if (data && typeof data.content === 'string') {
					paragraph.textContent = data.content;
				}
			} catch (e) {
				console.error('Message non JSON:', evt.data);
			}
		};

		ws.onclose = () => {
			statusEl.textContent = 'Déconnecté - reconnexion...';
			statusEl.className = 'status offline';
			setTimeout(connect, 1000); // tentative de reconnexion simple
		};

		ws.onerror = () => {
			statusEl.textContent = 'Erreur de connexion';
			statusEl.className = 'status error';
			ws.close();
		};
	}

	connect();
})();

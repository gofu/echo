package echo

//language=HTML
var indexTpl = []byte(`<html lang="en">
<head>
    <title>Echo</title>
</head>
<body>
<div>
    <p id="feedback"></p>
    <input type="text" id="address">
    <br>
    <img height="600" width="800" src="data:image/svg+xml,<svg xmlns=&quot;http://www.w3.org/2000/svg&quot;/>"
         alt="Browser screen" id="viewport">
</div>
<div>
    <button id="refresh"></button>
</div>
<script>
    let viewport = document.getElementById('viewport');
    let feedbackContainer = document.getElementById('feedback');
    let refreshButton = document.getElementById('refresh');
    let address = document.getElementById('address');

    function feedback(text) {
        if (!text) {
            feedbackContainer.innerText = '';
            feedbackContainer.style.display = 'none';
            return;
        }
        console.log(arguments);
        feedbackContainer.innerText = text;
        feedbackContainer.style.display = 'block';
    }

    feedback();

    let ws = new WebSocket("ws://127.0.0.1:3246/ws");
    ws.onerror = function (ev) {
        feedback('WS connect error', ev);
        refreshButton.onclick = null;
        address.onkeydown = null;
    };
    let messageID = 0;
    let queue = {};
    let listeners = {};
    let addListener = function (event, listener) {
        if (!listeners[event]) {
            listeners[event] = [];
        }
        listeners[event].push(listener);
        return function () {
            if (!listeners[event]) {
                return;
            }
            for (let i = 0; i < listeners[event].length; i++) {
                if (listeners[event][i] !== listener) {
                    continue;
                }
                listeners[event].splice(i, 1);
                break;
            }
            if (listeners[event].length === 0) {
                delete listeners[event];
            }
        };
    };
    let resetScreencastState = function () {
        refreshButton.innerText = "Start screencast";
        refreshButton.onclick = function () {
            let cleanup = addListener("Page.screencastFrame", function (data) {
                viewport.src = "data:text/plain;base64," + data.data;
                viewport.onload = function () {
                    messageID++;
                    ws.send(JSON.stringify({
                        id: messageID,
                        method: "Page.screencastFrameAck",
                        params: {sessionId: data.sessionId}
                    }));
                };
            });
            messageID++;
            ws.send(JSON.stringify({
                id: messageID,
                method: "Page.startScreencast"
            }));
            refreshButton.innerText = "Stop screencast";
            refreshButton.onclick = function () {
                cleanup();
                messageID++;
                ws.send(JSON.stringify({
                    id: messageID,
                    method: "Page.stopScreencast"
                }));
                resetScreencastState();
            };
        };
        address.onkeydown = function (ev) {
            if (ev.key !== "Enter") {
                return;
            }
            let url = address.value;
            if (url.length === 0) {
                return;
            }
            if (url.indexOf('://') === -1) {
                url = "http://" + url;
            }
            messageID++;
            ws.send(JSON.stringify({
                id: messageID,
                method: "Page.navigate",
                params: {url: url}
            }))
        };
    };
    ws.onopen = function () {
        resetScreencastState();
    };
    ws.onmessage = function (msg) {
        let data = JSON.parse(msg.data);
        if (data.method && listeners[data.method]) {
            for (let i = 0; i < listeners[data.method].length; i++) {
                listeners[data.method][i](data.params);
            }
        }
        if (data.id && queue[data.id]) {
            queue[data.id](data.result);
            delete queue[data.id];
            return;
        }
        console.log(data);
    };
</script>
</body>
</html>`)

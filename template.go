package echo

//language=HTML
var indexTpl = []byte(`<html lang="en">
<head>
    <title>Echo</title>
</head>
<body>
<div>
    <p id="feedback"></p>
    <img height="600" width="800" src="data:image/svg+xml,<svg xmlns=&quot;http://www.w3.org/2000/svg&quot;/>"
         alt="Browser screen" id="viewport">
</div>
<div>
    <button id="refresh">Refresh</button>
</div>
<script>
    let viewport = document.getElementById('viewport');
    let feedbackContainer = document.getElementById('feedback');
    let refreshButton = document.getElementById('refresh');

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
    };
    let messageID = 0;
    let queue = {};
    ws.onopen = function () {
        refreshButton.onclick = function () {
            messageID++;
            queue[messageID] = function (data) {
                viewport.src = "data:text/plain;base64," + data.data;
            };
            ws.send(JSON.stringify({
                id: messageID,
                method: "Page.captureScreenshot"
            }));
        };
    };
    ws.onmessage = function (msg) {
        let data = JSON.parse(msg.data);
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

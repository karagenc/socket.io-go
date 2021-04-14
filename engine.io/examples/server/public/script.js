const socket = new eio.Socket("http://127.0.0.1:3000");

const messageInput = $("#message-input");
const sendButton = $("#btn-send");
const status = $("#status");
const messageList = $("#message-list");

function send(e, message) {
  if (e && e.keyCode && e.keyCode !== 13) {
    return;
  }

  if (!message) {
    message = messageInput.val();
  }

  socket.send(message);
}

function addMessage(message) {
  messageList
    .append($(`<div class="message-wrapper"></div>`))
    .append($(`<span class="message-title">Message received: </span>`))
    .append($(`<span class="message-content"></span>`).text(message));
}

messageInput.keypress(send);
sendButton.click(send);

socket.on("open", () => {
  status.text(`Transport: ${socket.transport.name}`);
  socket.send("Hello from client");
});

socket.on("message", (message) => {
  addMessage(message);
});

socket.on("upgrade", (transport) => {
  status.text(`Transport: ${transport.name}`);
});

socket.on("close", (reason, desc) => {
  if (desc) {
    status.text(`Disconnected. Reason: ${reason} | ${desc}`);
  } else {
    status.text(`Disconnected. Reason: ${reason}`);
  }
});

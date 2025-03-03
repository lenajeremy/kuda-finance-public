import { useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { v4 } from "uuid";

import "./App.css";

function Message({ content }: { content: string }) {
  return (
    <Markdown
      components={{
        ol({ style, ...rest }) {
          return (
            <ol {...rest} style={{ ...style, listStylePosition: "inside" }} />
          );
        },
        ul({ style, ...rest }) {
          return (
            <ul {...rest} style={{ ...style, listStylePosition: "inside" }} />
          );
        },
      }}
      remarkPlugins={[remarkGfm]}
    >
      {content}
    </Markdown>
  );
}

function getConv(): string {
  const parts = location.pathname.slice(1).split("/");
  return parts[parts.length - 1];
}

function App() {
  const [input, setInput] = useState("");
  const conversationId = getConv();
  const [messages, setMessages] = useState<
    { content: string; role: "user" | "model"; id: string }[]
  >([]);
  const [mode, setMode] = useState<"chat" | "upload" | "home">(
    getConv() ? "chat" : "home"
  );
  const [stream, setStream] = useState("");
  const [loading, setLoading] = useState(false);
  const currentStreamValue = useRef("");

  useEffect(() => {
    fetch("/api/chat/" + conversationId)
      .then((d) => d.json())
      .then((d) => {
        setMessages(d.data);
      });
  }, [conversationId, mode]);

  const handleSubmit = async () => {
    setLoading(true);
    setInput("");
    if (!input.trim()) {
      return;
    }
    setMessages((prev) => [
      ...prev,
      { content: input, role: "user", id: v4() },
    ]);
    const es = new EventSource(
      "/api/chat?query=" +
        encodeURIComponent(input) +
        "&conversationId=" +
        conversationId
    );

    es.addEventListener("message", (e) => {
      setLoading(false);
      currentStreamValue.current += (e.data as string).slice(
        1,
        e.data.length - 1
      );
      setStream(currentStreamValue.current);
    });

    es.addEventListener("end", () => {
      setLoading(false);
      const n = {
        content: currentStreamValue.current,
        role: "model" as "user" | "model",
        id: v4(),
      };

      setMessages((prev) => [...prev, n]);
      currentStreamValue.current = "";
      setStream("");
      es.close();
    });

    es.addEventListener("error", () => {
      setLoading(false);
    });
  };

  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current
        .querySelector(".invisible-message")
        ?.scrollIntoView();
    }
  }, [messages]);

  const handleStart = async () => {
    let conversationId = localStorage.getItem("lastConversation");
    if (!conversationId) {
      const res = await fetch("/api/chat/new", { method: "POST" });
      const data = await res.json();
      conversationId = data.conversation.id as string;
      localStorage.setItem("lastConversation", conversationId);
    }

    location.href = `/conversation/${conversationId}`;
  };

  if (mode === "home") {
    return (
      <div style={{ textAlign: "center" }}>
        <p>Welcome</p>
        <button onClick={handleStart}>Start</button>
      </div>
    );
  }

  return (
    <div>
      <div
        style={{
          display: "flex",
          marginTop: "2rem",
          gap: "1rem",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <button onClick={() => setMode("chat")}>
          {mode == "chat" && "*"}Chat
        </button>
        <button onClick={() => setMode("upload")}>
          {mode == "upload" && "*"}Upload
        </button>
      </div>
      {mode == "chat" && (
        <form
          style={{ overflowY: "scroll" }}
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
        >
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: "0.5rem",
              overflowY: "scroll",
              scrollBehavior: "smooth",
            }}
            ref={containerRef}
          >
            {messages.map((m) => (
              <div
                key={m.id}
                style={{
                  padding: "0.75rem 1rem",
                  background: m.role === "model" ? "#363636" : "black",
                  borderRadius: "2rem",
                  width: "fit-content",
                  whiteSpace: "pre-line",
                  maxWidth: "85%",
                  [m.role === "model" ? "marginRight" : "marginLeft"]: "auto",
                }}
                className="message"
              >
                <Message content={m.content} />
              </div>
            ))}
            {stream && (
              <div
                style={{
                  padding: "0.75rem 1rem",
                  background: "#363636",
                  borderRadius: "2rem",
                  width: "fit-content",
                  whiteSpace: "pre-line",
                  maxWidth: "85%",
                  marginRight: "auto",
                }}
              >
                {stream}
              </div>
            )}
            {loading && (
              <div
                style={{
                  padding: "0.75rem 1rem",
                  background: "#363636",
                  borderRadius: "2rem",
                  width: "fit-content",
                  maxWidth: "85%",
                  marginRight: "auto",
                }}
              >
                * * *
              </div>
            )}
            <div className="invisible-message" />
          </div>
          <div className="input-container">
            <textarea
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  handleSubmit();
                }
              }}
              placeholder="Hello puny human"
              value={input}
              onChange={(e) => setInput(e.currentTarget.value)}
            />
            <button type="submit">send</button>
          </div>
        </form>
      )}

      {mode == "upload" && (
        <form
          method="POST"
          encType="multipart/form-data"
          action={"/api/upload"}
        >
          <input type="file" name="statementDoc" />
          <button type="submit">SUBMIT</button>
        </form>
      )}
    </div>
  );
}

export default App;

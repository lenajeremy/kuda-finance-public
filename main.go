package main

import (
	"awesomeProject/ai"
	"awesomeProject/db"
	"awesomeProject/graph"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"gorm.io/gorm"
)

//go:embed frontend/dist
var embeddedFiles embed.FS

func init() {
	// if err := godotenv.Load(); err != nil {
	// 	panic("failed to load env")
	// }

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

func main() {
	model := ai.New()
	conn, err := graph.NewGraphConn()
	if err != nil {
		slog.Debug("error connecting to neo4j")
		panic(err)
	}
	defer conn.Close()

	sqlite := db.New()

	err = sqlite.AutoMigrate(&db.Conversation{}, &db.Message{})
	if err != nil {
		slog.Error("error migrating database", "error", err.Error())
	}

	r := gin.Default()

	distFs, err := fs.Sub(embeddedFiles, "frontend/dist")
	if err != nil {
		slog.Error("failed to embed file system", "error", err.Error())
		panic(err)
	}
	httpFs := http.FS(distFs)

	var allowedURLs = map[string]bool{"/": true}

	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", httpFs)
	})

	r.NoRoute(func(c *gin.Context) {
		if allowedURLs[c.Request.URL.Path] || strings.HasPrefix(c.Request.URL.Path, "/conversation") {
			c.FileFromFS("/", httpFs)
			return
		}
		c.JSON(404, gin.H{"error": "NOT FOUND"})
	})

	assetsFs, err := fs.Sub(distFs, "assets")
	if err != nil {
		slog.Error("failed to create assets file system", "error", err.Error())
		panic("failed to create assets filesystem")
	}
	r.StaticFS("/assets", http.FS(assetsFs))

	r.GET("/list", func(c *gin.Context) {
		output := ""
		fs.WalkDir(distFs, ".", func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				fmt.Printf("Error accessing %s: %v\n", path, err)
				return err
			}
			output += fmt.Sprintln("Path:", path)
			if info.IsDir() {
				output += fmt.Sprintln("Type: Directory")
			} else {
				output += fmt.Sprintln("Type: File")
			}
			return nil
		})

		c.JSON(200, gin.H{"information": output})
	})

	aimodel := model.GenerativeModel("gemini-2.0-pro-exp")
	cs := aimodel.StartChat()

	api := r.Group("/api")

	api.GET("/health", func(c *gin.Context) {
		c.String(200, "OK")
	})

	api.GET("/chat/:chat-id", func(c *gin.Context) {
		conversationId := c.Param("chat-id")
		if conversationId == "" {
			c.JSON(400, gin.H{"error": "invalid conversation"})
			return
		}

		var messages []*db.Message
		tx := sqlite.Find(&messages, "conversation_id = ?", conversationId).Order("created_at ASC")

		if err != nil {
			if errors.Is(tx.Error, gorm.ErrRecordNotFound) || tx.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "conversation with id not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error: failed to retrieve conversation"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"error": nil, "data": messages, "count": len(messages)})
	})

	api.POST("/upload", func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": 400, "error": "no file"})
			return
		}

		statementDocs, ok := form.File["statementDoc"]
		if !ok || len(statementDocs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": 400, "error": "no file"})
			return
		}

		statementFile, err := statementDocs[0].Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": 500, "error": "failed to open file"})
		}

		transactions := parseFile(statementFile)

		var totalIn, totalOut float64
		for _, t := range transactions {
			if t.Type == -1 {
				totalOut += t.Amount
			} else {
				totalIn += t.Amount
			}
		}

		fmt.Printf("Total In: %f; Total Out: %f\n", totalIn, totalOut)

		for _, t := range transactions {
			category, err := model.PredictCategory(t.String())
			if err != nil {
				slog.Error("error: predict category error", "transaction", t.String(), "error", err)
				continue
			}
			t.Category = category
			err = saveTransaction(t)
			if err != nil {
				slog.Error("error: saving category", "transaction", t.String(), "error", err)
				continue
			}
			time.Sleep(time.Millisecond * 1000)
		}

		c.JSON(200, gin.H{"done": true})
	})

	api.POST("/chat/new", func(c *gin.Context) {
		conversation := db.Conversation{}
		tx := sqlite.Create(&conversation)
		if tx.Error != nil {
			c.JSON(500, gin.H{"message": "failed to create conversation"})
			return
		}

		c.JSON(200, gin.H{"conversation": conversation})
	})

	api.GET("/chat", func(c *gin.Context) {
		query := c.Query("query")
		conversationId := c.Query("conversationId")

		log.Println(query)

		if query == "" {
			slog.Error("no query provided")
			c.JSON(http.StatusBadRequest, gin.H{
				"response": nil,
				"error":    "no query provided",
			})
			return
		}

		if conversationId == "" {
			slog.Error("no conversation id provided")
			c.JSON(http.StatusBadRequest, gin.H{
				"response": nil,
				"error":    "no conversation id provided",
			})
			return
		}

		cypher, err := model.GenerateCypher(query)
		if err != nil {
			slog.Error("error generating cypher", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"response": nil,
				"error":    fmt.Errorf("failed to generate cypher: %s", err.Error()),
			})
			return
		}

		res, err := conn.Execute(context.Background(), cypher, map[string]any{})
		if err != nil {
			slog.Error("error running query", "error", err.Error())
			res = &neo4j.EagerResult{}
		}

		conversation := db.Conversation{}
		tx := sqlite.Model(&db.Conversation{}).Preload("Messages").Where("id = ?", conversationId).First(&conversation)
		if tx.Error != nil {
			c.JSON(500, gin.H{"message": "conversation id not found"})
			return
		}

		response := model.Respond(query, res.Records, conversation.Messages, cs)

		c.Stream(func(w io.Writer) bool {
			r, err := response.Next()
			if err != nil {
				c.SSEvent("end", "close connection")
				slog.Error("error streaming ai response", "error", err.Error())
				return false
			}
			data := fmt.Sprintf(`"%s"`, r.Candidates[0].Content.Parts[0])
			slog.Info(data)
			c.SSEvent("message", data)
			return true
		})

		newMessage := string(response.MergedResponse().Candidates[0].Content.Parts[0].(genai.Text))
		err = sqlite.Transaction(func(tx *gorm.DB) error {
			err := tx.Create(&db.Message{Content: query, Role: db.ROLEUSER, ConversationId: conversation.ID}).Error
			if err != nil {
				return err
			}

			err = tx.Create(&db.Message{Content: newMessage, Role: db.ROLEBOT, ConversationId: conversation.ID}).Error
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			slog.Error("error", "error", err.Error())
			c.AbortWithError(500, err)
		}
	})

	r.Run(":8080")

	// http.Handle("/web/", http.StripPrefix("/web", http.FileServer(http.Dir("./frontend/dist"))))
	// // http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// // 	fmt.Fprintf(w, "Hello, World!")
	// // })

	// log.Fatal(http.ListenAndServe(":8080", nil))
}

package main

import (
	"awesomeProject/graph"
	"bufio"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
)

type Transaction struct {
	DateTime    time.Time `json:"transactionTime"`
	Amount      float64   `json:"amount"`
	Type        int       `json:"-"`
	TypeString  string    `json:"type"`
	Category    string
	Party       string
	Description string
	Balance     float64
}

func (t Transaction) String() string {
	var transactionType string
	if t.Type == -1 {
		transactionType = "DEBIT"
	} else {
		transactionType = "CREDIT"
	}

	return fmt.Sprintf("%s: Date: %s; Amount: %.2f; Party: %s; Description: %s", transactionType, t.DateTime, t.Amount, t.Party, t.Description)
}

func parseFile(file multipart.File) []*Transaction {
	reader := bufio.NewScanner(file)
	transactions := make([]*Transaction, 0)

	for reader.Scan() {
		var line = reader.Text()

		t, err := parseLine(line)
		if err != nil {
			log.Println("failed to parse line", line)
		} else {
			transactions = append(transactions, t)
		}
	}
	return transactions
}

func parseLine(line string) (*Transaction, error) {
	var fields = splitLine(line)
	timeStr, amountStr, category, party, description, balanceStr := fields[0], fields[1], fields[3], fields[4], fields[5], fields[6]

	if amountStr == string(rune(9)) {
		amountStr = fields[2]
	}

	timeStr = strings.Trim(timeStr, string(rune(9)))
	amountStr = strings.Trim(amountStr[3:], string(rune(9))) // starting from index 3 to remove the ₦ character
	amountStr = strings.ReplaceAll(amountStr, ",", "")

	balanceStr = strings.Trim(balanceStr[3:], string(rune(9)))
	balanceStr = strings.ReplaceAll(balanceStr, ",", "")

	category = strings.Trim(category, string(rune(9)))
	party = strings.Trim(party, string(rune(9)))
	description = strings.Trim(description, string(rune(9)))

	dateTime, err := time.Parse("02/01/06 15:04:05", timeStr)
	if err != nil {
		return nil, fmt.Errorf("%s:%s", "invalid date", err.Error())
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil, fmt.Errorf("%s:%s", "invalid amount", err.Error())
	}

	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("%s:%s", "invalid balance", err.Error())
	}

	var transaction = Transaction{
		DateTime:    dateTime,
		Amount:      amount,
		Category:    category,
		Party:       party,
		Description: description,
		Balance:     balance,
	}

	if fields[2] != string(rune(9)) {
		transaction.Type = -1
		transaction.TypeString = "Debit"
	} else {
		transaction.Type = +1
		transaction.TypeString = "Credit"
	}

	return &transaction, nil
}

func splitLine(line string) []string {
	spacesCount := 0
	return strings.FieldsFunc(line, func(r rune) bool {
		if r == 9 {
			spacesCount += 1
		}

		if spacesCount == 2 {
			spacesCount = 0
			return true
		}
		return false
	})
}

func createCategories() {
	category := []string{
		"Family",
		"Girlfriend",
		"Food",
		"Internet/Airtime",
		"Clothing",
		"Debt",
		"Cowrywise In",
		"Cowrywise Out",
		"Electricity Bill",
		"Miscellaneous",
		"Church",
		"Transportation",
		"Personal Care",
		"Subscriptions",
		"Drinks",
		"LoanPayment-Out",
		"LoanPayment-In",
		"LoanRepayment-Out",
		"LoanRepayment-In",
		"Salary",
	}

	conn, err := graph.NewGraphConn()
	if err != nil {
		panic(fmt.Errorf("failed to connect to graph: %s", err.Error()))
	}
	defer conn.Close()

	for _, cat := range category {
		_, err := conn.Execute(
			context.Background(),
			`
			CREATE (c:Category {name: $name})
 			RETURN c
			`,
			map[string]interface{}{"name": cat})
		if err != nil {
			panic(err)
		}
	}
}

// saveTransaction saves a transaction to the database
func saveTransaction(t *Transaction) error {
	graphConn, err := graph.NewGraphConn()
	if err != nil {
		return fmt.Errorf("failed to connect to graph database: %s", err.Error())
	}
	defer graphConn.Close()

	query := `
	MERGE (c:Category {name: $category})
	CREATE (t:Transaction {dateTime: $dateTime, amount: $amount, type: $type, party: $party, description: $description, balance: $balance})
	MERGE (t)-[:BELONGS_TO]->(c)
	RETURN t`

	params := map[string]interface{}{
		"dateTime":    t.DateTime,
		"amount":      t.Amount,
		"type":        t.TypeString,
		"category":    t.Category,
		"party":       t.Party,
		"description": t.Description,
		"balance":     t.Balance,
	}

	res, err := graphConn.Execute(context.Background(), query, params)
	if err != nil {
		return fmt.Errorf("failed to execute query: %s", err.Error())
	}

	summary := res.Summary
	counters := summary.Counters()
	fmt.Println("Nodes created", counters.NodesCreated())
	fmt.Println("Labels added", counters.LabelsAdded())
	fmt.Println("Properties set:", counters.PropertiesSet())
	fmt.Println("Relationships created", counters.RelationshipsCreated())
	return nil
}

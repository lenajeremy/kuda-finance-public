package ai

import (
	"awesomeProject/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"google.golang.org/api/option"

	"log"
	"os"
)

type AI struct {
	*genai.Client
}

func New() *AI {
	apiKey := os.Getenv("GEMINI_API_KEY")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		slog.Error("failed to create ai client", "error", err.Error())
		return nil
	}

	return &AI{
		client,
	}
}

func (ai AI) PredictCategory(s string) (string, error) {
	prompt := `You are a financial expert who is very proficient in your job. Right now you're tasked with the responsibility of analysing a transaction and deciding 
the category of the transaction. If you fail at your task, you'd be sacked and you'd starve. So you need to think critically before answering.

<RelevantContext>
The user's name is X X, he is based in Lagos, Nigeria.
His girlfriend's name is X X.
These are the names of his family members
 - X X
 - X X
 - X X
 - X X

The following people are food vendors
 - X X
 - X X
 - X X
 - X X
 - X X
</RelevantContext>

It's your job to take a look at the details of the transaction, the date, the party, amount, description
to ascertain the category in category to which the transaction belongs.

These are the expected transactions
1. Family
2. Girlfriend
3. Food
4. Internet/Airtime
5. Clothing
6. Debt
7. Cowrywise In
8. Cowrywise Out
9. Electricity Bill
10. Miscellaneous
11. Church
12. Transportation
13. Personal Care
14. Subscriptions
15. Drinks
16. LoanPayment-Out (for when it's a loan-related transaction and I'm the debtor. It has a to be a debit transaction)
17. LoanPayment-In (for when it's a loan-related transaction and I'm the creditor. It has to be a credit transaction)
18. LoanRepayment-Out (for when I repay a loan and I'm the debtor. It has to be a debit transaction)
19. LoanRepayment-In (for when I receive payment for a loan and I'm the creditor. It has to be a credit transaction)
20. Salary

Note, your preference should be to check who the money was sent to or who the money is from.
Then, the decipher the category from the transaction description.
Also, use the type of the transaction (CREDIT to indicate that the money is coming into my account, and DEBIT to indicate that the money is leaving my account)
If you cannot ascertain the category from each, return "UNKNOWN".

<RelevantExamples>
<Example>
	<Transaction>	
		DEBIT; Amount: 5000; Party: Michael  Arowolo/8095609306/Opay Digital Services Limited amount; Description: clothes
	</Transaction>
	<ExpectedResponse>
		Clothing
	</ExpectedResponse>
	<Why>
		From the description, it shows that this is for clothes
	</Why>
</Example>
<Example>
	<Transaction>
		DEBIT Date: 10/5/2024; Amount: 600; Party: ""; Description: 2.5gb for 2 days purchase    
	</Transaction>
	<ExpectedResponse>
		Internet & Airtime
	</ExpectedResponse>
	<Why>
		From the description, you can clearly see the purchase of 2.5gb data for 2 days
	</Why>
</Example>

<Example>
	<Transaction>
		DEBIT; Date: 10/5/2024; Amount: 2000; Party: Joy Elu Odama/9118297388/Paycom(Opay); Description: hellll
	</Transaction>
	<ExpectedResponse>
		Feeding
	</ExpectedResponse>
	<Why>
		From the provided context, Joy Elu Odama is a food vendor. Since this is a debit transaction, 
		it's most likely a food purchase.
	</Why>
</Example>
<Example>
	<Transaction>
		DEBIT: Date: 2025-01-01 17:35:11 +0000 UTC; Amount: 3000.00; Party: Jeremiah Osaigbokan Lena/2123379333/United Bank For Africa; Description: stuff
	</Transaction>
	<ExpectedResponse>
		Unknown
	</ExpectedResponse>
	<Why>
		This is clearly an outward transfer, with a very vague description, so the category should be unknown
	</Why>
</Example>
<Example>
	<Transaction>
		CREDIT: Date: 2025-01-02 18:57:05 +0000 UTC; Amount: 5000.00; Party: Damilola Victoria Odeogberin/7048478064/Paycom(Opay); Description: thanks for coming through babeeeee (loan return)
	</Transaction>
	<ExpectedResponse>
		LoanRepayment-Crdt
	</ExpectedResponse>
	<Why>
		This is a Credit transaction. Indicating that this is payment for a loan for which I'm the creditor.
	</Why>
</Example>
<Example>
	<Transaction>
		DEBIT: Date: 2025-01-03 13:07:19 +0000 UTC; Amount: 2100.00; Party: Hopeful Okere Uchechi International Business Ventures - Hope Business Ventures1/8203189697/Moniepoint Mfb; Description: charger
	</Transaction>
	<ExpectedResponse>
		Miscellaneous
	</ExpectedResponse>
	<Why>
		This is a debit transaction. and the purchase is a charger, which is not vague. this should be marked as Miscellaneous
	</Why>
</Example>
<Example>
	<Transaction>
		DEBIT: Date: 2025-01-23 17:24:25 +0000 UTC; Amount: 2010.00; Party: Pos Transfer-Fatimoh Gbolahan Mudasiru/5877941385/Moniepoint Mfb; Description: beans and eggs
	</Transaction>
	<ExpectedResponse>
		Feeding
	</ExpectedResponse>
	<Why>
		Although we don't know who Fatimoh Gbolahan Mudashiru is, we know that bread and eggs are food stuffs
	</Why>
</Example>
</RelevantExamples>

<Important>
1. It's very important your response is a valid category (provided above).
2. It's very important your response is a single word (the category).
</Important>`
	model := ai.GenerativeModel("gemini-2.0-pro-exp")
	cs := model.StartChat()
	cs.History = []*genai.Content{
		{
			Parts: []genai.Part{
				genai.Text(prompt),
			},
			Role: "user",
		},
	}

	res, err := cs.SendMessage(context.Background(), genai.Text(s))
	if err != nil {
		return "", fmt.Errorf("error getting chat completion: %v", err)
	}

	category, ok := res.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "UNKNOWN", nil
	}
	return strings.TrimSpace(string(category)), nil
}

func (ai *AI) GenerateCypher(query string) (string, error) {
	prompt := `
	You're a expect cypher query generator. You're extremely proficient at your job. 
	Your job is to take a user's query and generate cypher queries that'd return results
	that'd help answer the user's query. You take every thing into consideration before 
	creating the cypher query. You're very good at your job and you're very confident in your abilities.

	This the structure of the graph database:
	<DatabaseVisualization>

	Node: Transaction
		- amount: Double
		- balance: Double
		- dateTime: DateTime
		- description: String
		- party: String
		- type: String (Credit or Debit)
	
	Node: Category
		- name: String
	
	Relationships:
		- BELONGS_TO (Transaction) -> (Category)

	Categories:
		- Family
		- Girlfriend
		- Food
		- Internet/Airtime
		- Clothing
		- Debt
		- Electricity Bill
		- Miscellaneous
		- Church
		- Transportation
		- Personal Care
		- Subscriptions
		- Drinks
		- LoanPayment-Out
		- LoanPayment-In
		- LoanRepayment-Out
		- LoanRepayment-In
		- Salary

	</DatabaseVisualization>

	<Important>
	- You should only respond with only valid cypher queries. Do not make any comments or any other text. Your response is meant to be run directly
	against the graph database, so it has to be flawless.
	- Ensure that there are no syntax errors in your query. Take time to think through your query before responding.
	- Ensure that your response is a valid cypher query.
	- Your cypher query should only return the graph nodes.
	- Your response should never contain any form of formatting by putting in quotes, backticks or adding the "cypher" before it. return only the valid query. it's very important that your response can be run on a real database without any error. 
	- Again, no form of formatting is required.
	</Important>

	<Important>
	DO NOT HALLUCINATE. Only generate queries that you are sure would work. think critically about the instructions above before responding. it's crucial.
	</Important>

	<Examples>
	1. 
	<Query>
	How much have I spent on food this month?
	</Query>

	<Expected Response>
	MATCH (t:Transaction)-[:BELONGS_TO]->(c:Category {name: "Food"}) WHERE t.dateTime >= datetime("2025-01-01") AND t.dateTime <= datetime("2025-01-31") RETURN t
	</Expected Response>

	<Explanation>
	Since the user is asking for how much they've spent on food this month, you need to know which month it currently is.
	It's February. So you need to get all transactions that belong to the category "Food" and that happened between the first and last day of the month.

	Note: You should use the datetime() function because the dateTime property is a DateTime type.
	It's very important to use the correct function for the correct data type.
	</Explanation>


	2. 
	<Query>
	Did I pay for electricity last month? How much did I pay?
	</Query>
	<ExpectedResponse>
	MATCH (t:Transaction)-[:BELONGS_TO]->(c:Category {name: "Electricity Bill"}) WHERE t.dateTime >= datetime("2025-01-01") AND t.dateTime <= datetime("2025-01-31") RETURN t
	</ExpectedResponse>
	<Explanation>
	- The user is asking if they paid for electricity last month and how much they paid.
	- You need to know which month they're asking for. Since, we're in february, the user is asking for January.
	- So you should check if there are any transactions that belong to the category "Electricity Bill" that happened in January.
	- If there are, you should return all the transactions. 
	</Explanation>


	3. 
	<Query>
	How much has my girlfriend sent to me this month?
	</Query>
	<ExpectedResponse>
	MATCH (t:Transaction)-[:BELONGS_TO]->(c:Category {name: "Girlfriend"}) WHERE t.dateTime >= datetime("2025-01-01") AND t.dateTime <= datetime("2025-01-31") AND t.type = "Credit" RETURN t
	</ExpectedResponse>
	<Explanation>
	- The user is asking how much their girlfriend has sent to them this month.
	- You need to know which month they're asking for. Since, we're in february, the user is asking for January.
	- So you should check if there are any transactions that belong
	to the category "Girlfriend" that happened in January.
	- You should also check if the transaction type is "Credit" because the user is asking for how much was sent to them.
	- If there are, you should return the transactions.
	</Explanation>


	4. 
	<Query>
	How much have I sent to my girlfriend this month?
	</Query>
	<ExpectedResponse>
	MATCH (t:Transaction)-[:BELONGS_TO]->(c:Category {name: "Girlfriend"}) WHERE t.dateTime >= datetime("2025-01-01") AND t.dateTime <= datetime("2025-01-31") AND t.type = "Debit" RETURN t
	</ExpectedResponse>
	<Explanation>
	- The user is asking how much they sent their girlfriend this month.
	- You need to know which month they're asking for. Since, we're in february, the user is asking for January.
	- So you should check if there are any transactions that belong
	to the category "Girlfriend" that happened in January.
	- You should also check if the transaction type is "Debit" because the user is asking for how much they sent.
	- If there are, you should return the transactions.
	</Explanation>

	5.
	<Query>
	as at the 19 of last month, how much had i spent? compare that to how much i've spent this month
	</Query>
	<ExpectedResponse>
	MATCH (t:Transaction)-[:BELONGS_TO]->(c:Category) WHERE t.dateTime >= datetime("2025-01-01") AND t.dateTime <= datetime("2025-01-19") AND t.type = "Debit"
	MATCH (t2:Transaction)-[:BELONGS_TO]->(c:Category) WHERE t2.dateTime >= datetime("2025-02-01") AND t2.dateTime <= datetime("2025-02-19") AND t2.type = "Debit"
	RETURN t, t2
	</ExpectedResponse>
	<Explanation>
	The user is asking for a comparison between the amount spent as at the 19th of last month and the amount spent as at the 19th of this month.
	You need to get all transactions that happened between the 1st and 19th of last month and all transactions that happened between the 1st and 19th of this month.
	You should return both sets of transactions.
	</Explanation>
	</Examples>

	Note:
	you can also create queries by searching fields like description, party, amount, etc.
	MATCH (t:Transaction) WHERE t.description CONTAINS 'rent' RETURN t
	MATCH (t:Transaction) WHERE t.party CONTAINS 'john doe' RETURN t
	MATCH (t:Transaction) WHERE t.amount > 5000 RETURN t
	if the user query is asking for a specific date or dates, you should ensure that the date is correct and valid. considering leap years and the number of days in each month.
	if the user is asking for a particular person's name, ensure that you convert the search name to lower case in order for it to match any form of the name string, eg
	MATCH (t:Transaction)
	WHERE toLower(t.party) CONTAINS toLower("user's name") AND t.type = "Credit"
	RETURN t

	if the user's query is unrelated to transactions or their account details. you should return:
	match (c:Category{name:"empty"})
	return c

	<Important>
	DO NOT TRY TO HOLD A CONVERSATION, RESPOND USING SENTENSE. YOUR ONLY RESPONSE SHOULD BE VALID CYPHER QUERIES.
	YOU'D BE PENALIZED IF YOU DO ANYTHING OTHER THAN THIS.
	</Important>
	`
	model := ai.GenerativeModel("gemini-2.0-pro-exp")
	cs := model.StartChat()
	cs.History = []*genai.Content{
		{
			Parts: []genai.Part{
				genai.Text(prompt),
			},
			Role: "user",
		},
	}

	res, err := cs.SendMessage(context.Background(), genai.Text(query))
	if err != nil {
		return "", fmt.Errorf("error getting chat completion: %v", err)
	}

	cypher, ok := res.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "UNKNOWN", nil
	}

	withoutCypherPretext := strings.TrimSuffix(strings.TrimPrefix(string(cypher), "```cypher"), "```")
	println(withoutCypherPretext)
	return strings.TrimSpace(withoutCypherPretext), nil
}

func (ai *AI) Respond(query string, rec []*neo4j.Record, prevMessages []db.Message, cs *genai.ChatSession) *genai.GenerateContentResponseIterator {

	if len(prevMessages) == 0 {
		cs.History = []*genai.Content{
			{
				Parts: []genai.Part{
					genai.Text(`
					your job is to answer the user's query. 
					you'll be provided the context from which to answer these questions. 
					respond to the user's query as accurately as you posisbly can using the provided information. 
	
					ensure that you critically analyse each transaction and provide the most accurate response possible.
					ensure that you do not hallucinate. carefully perform math operations when required. and use the calculator when necessary.
	
					You should sound as free and human as possible, not like a robot. default currency is in naira. dates should be described properly
				`),
				},
				Role: "user",
			},
		}
	} else {
		history := make([]*genai.Content, 0)
		for _, message := range prevMessages {
			history = append(history, &genai.Content{
				Parts: []genai.Part{
					genai.Text(message.Content),
				},
				Role: string(message.Role),
			})
		}
		cs.History = history
	}

	recordString := ""
	for _, r := range rec {
		for _, v := range r.Values {
			value := v.(neo4j.Node)
			jsonString, err := json.MarshalIndent(value.Props, " ", " ")
			if err != nil {
				log.Printf("Error marshalling record: %v", err)
			}
			recordString += string(jsonString) + "\n\n"
		}
	}

	query = fmt.Sprintf(
		`<Query>%s</Query> 

		<RelevantContext>%s</RelevantContext>`,
		query,
		recordString,
	)

	res := cs.SendMessageStream(context.Background(), genai.Text(query))
	return res
}

func (ai *AI) Close() error {
	return ai.Client.Close()
}

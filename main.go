package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/henomis/lingoose/llm/openai"
	"github.com/henomis/lingoose/pipeline"
	sqlpipeline "github.com/henomis/lingoose/pipeline/sql"
	"github.com/henomis/lingoose/prompt"
	"github.com/henomis/lingoose/types"
	"github.com/jedib0t/go-pretty/v6/table"
	_ "github.com/mattn/go-sqlite3"
)

func main() {

	var dataSourceType string
	var dataSourceName string
	var question string
	var plotQuestion string
	var openaiApiKey string

	flag.StringVar(&dataSourceType, "t", "", "type of the datasource (sqlite|mysql)")
	flag.StringVar(&dataSourceName, "n", "", "name of the datasource (database path|connection string)")
	flag.StringVar(&question, "q", "", "question to ask the datasource")
	flag.StringVar(&plotQuestion, "p", "", "question to plot the datasource")
	flag.StringVar(&openaiApiKey, "k", "", "openai api key (defaults to OPENAI_API_KEY env var)")

	flag.Parse()

	var sqlDataSource sqlpipeline.DataSourceType
	if dataSourceType == "sqlite" {
		sqlDataSource = sqlpipeline.DataSourceSqlite
	} else if dataSourceType == "mysql" {
		sqlDataSource = sqlpipeline.DataSourceSqlite
	} else if openaiApiKey == "" {
		openaiApiKey = os.Getenv("OPENAI_API_KEY")
		if openaiApiKey == "" {
			fmt.Println("Please provide an OpenAI API key")
			os.Exit(1)
		}
	} else {
		fmt.Println("Please provide a datasource type")
		os.Exit(1)
	}

	if dataSourceName == "" {
		fmt.Println("Please provide a datasource name")
		os.Exit(1)
	} else if question == "" {
		fmt.Println("Please provide a question")
		os.Exit(1)
	}

	sqlPipe, err := sqlpipeline.New(
		openai.NewCompletion().WithMaxTokens(1000).WithTemperature(0),
		sqlDataSource,
		dataSourceName,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	answer, err := sqlPipe.Run(context.Background(), types.M{"question": question})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	description := answer["output"].(string)
	sqlQuery := answer["sql_query"].(string)
	sqlResult := answer["sql_result"].(string)

	fmt.Println()
	renderQuestion(question)
	renderSQLQuery(sqlQuery)
	renderDescription(description)
	renderSQLResultTable(sqlResult)

	if plotQuestion != "" {
		err = plotDataSource(plotQuestion, sqlResult)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

}

func renderQuestion(question string) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendHeader(table.Row{"SQL QUESTION"})
	t.AppendRows([]table.Row{{question}})
	t.SetStyle(table.StyleColoredBlackOnYellowWhite)
	t.Render()
	fmt.Println()
}

func renderSQLQuery(sqlQuery string) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendHeader(table.Row{"SQL QUERY"})
	t.AppendRows([]table.Row{{sqlQuery}})
	t.SetStyle(table.StyleColoredBlackOnGreenWhite)
	t.Render()
	fmt.Println()
}

func renderDescription(description string) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendHeader(table.Row{"DESCRIPTION"})
	t.AppendRows([]table.Row{{description}})
	t.SetStyle(table.StyleColoredBlackOnMagentaWhite)
	t.Render()
	fmt.Println()
}

func renderSQLResultTable(sqlResult string) {

	sqlResultRows := strings.Split(sqlResult, "\n")

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	for i, row := range sqlResultRows {

		cols := strings.Split(row, "|")

		var tableRow table.Row
		for _, col := range cols {
			tableRow = append(tableRow, col)
		}

		if i == 0 {
			t.AppendHeader(tableRow)
		} else {
			t.AppendRow(tableRow)
		}
	}

	t.SetStyle(table.StyleColoredBlackOnCyanWhite)
	t.Render()
	fmt.Println()
}

func sqlResultTableToMarkdown(sqlResult string) string {

	markdownTable := ""
	sqlResultRows := strings.Split(sqlResult, "\n")

	for i, row := range sqlResultRows {

		cols := strings.Split(row, "|")

		if i == 0 {
			markdownTable += "|"
			for _, col := range cols {
				markdownTable += col + "|"
			}
			markdownTable += "\n|"
			for i := 0; i < len(cols); i++ {
				markdownTable += "---|"
			}
			markdownTable += "\n"
		} else {
			markdownTable += "|"
			for _, col := range cols {
				markdownTable += col + "|"
			}
			markdownTable += "\n"
		}
	}

	return markdownTable
}

func sqlResultTableToHTML(sqlResult string) string {

	htmlTable := "<br><table border=\"1\" style=\"border-collapse: collapse\">"
	sqlResultRows := strings.Split(sqlResult, "\n")

	for i, row := range sqlResultRows {

		cols := strings.Split(row, "|")

		if i == 0 {
			htmlTable += "<tr>"
			for _, col := range cols {
				htmlTable += "<th>" + col + "</th>"
			}
			htmlTable += "</tr>"
		} else {
			htmlTable += "<tr>"
			for _, col := range cols {
				htmlTable += "<td>" + col + "</td>"
			}
			htmlTable += "</tr>"
		}
	}

	htmlTable += "</table>"

	return htmlTable
}

func plotDataSource(plotQuery, sqlResult string) error {

	chartJS := pipeline.Llm{
		LlmEngine: openai.NewCompletion().WithMaxTokens(1000).WithTemperature(0),
		LlmMode:   pipeline.LlmModeCompletion,
		Prompt:    prompt.NewPromptTemplate("Consider the following markdown data table:\n\n{{.sql_result}}\n\ncreate a chart.js canvas and script elements to plot such data. don't add other html elements.\n{{.plot_query}}"),
	}
	pipeChartJS := pipeline.NewTube(chartJS)

	output, err := pipeChartJS.Run(context.Background(), types.M{"sql_result": sqlResultTableToMarkdown(sqlResult), "plot_query": plotQuery})
	if err != nil {
		return err
	}

	chartFile, err := os.Create("chart.html")
	if err != nil {
		return err
	}

	header := `<html><head><title>Chart.js Plot</title><script src="https://cdn.jsdelivr.net/npm/chart.js@2.8.0"></script></head><body>`

	chartFile.Write([]byte(header))
	chartFile.Write([]byte(output["output"].(string)))
	chartFile.Write([]byte(sqlResultTableToHTML(sqlResult)))
	chartFile.Write([]byte("</body></html>"))

	return nil
}

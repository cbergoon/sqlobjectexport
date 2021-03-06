package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
)

type config struct {
	Directory  string `json:"directory"`
	DbAddress  string `json:"dbAddress"`
	DbPort     string `json:"dbPort"`
	DbUsername string `json:"dbUsername"`
	DbPassword string `json:"dbPassword"`
	DbDatabase string `json:"dbDatabase"`
	DbSchema   string `json:"dbSchema"`
	DbjectType string `json:"dbjectType"`
	Git        bool   `json:"git"`
	GitAddress string `json:"gitAddress"`
}

type Object struct {
	SchemaName          string
	ObjectId            int
	ObjectName          string
	ObjectType          string
	ObjectTypeDesc      string
	ObjectDefinition    string
	RetrievedDefinition bool
}

func (o *Object) generateCommentBlock() string {
	disclaimer := ""
	if strings.TrimSpace(o.ObjectType) == "U" {
		disclaimer = "NOTE: User table is generated as table variable for reference purposes only"
	} else {
		disclaimer = "NOTE: Object is exported with definition only"
	}
	block := `/* 
	%s Object Generated by sqlobjectdump

	%s.%s

	%s
*/

`
	return fmt.Sprintf(block, o.ObjectTypeDesc, o.SchemaName, o.ObjectName, disclaimer)
}

type ObjectDefinition struct {
	Definition []string
}

func (od *ObjectDefinition) String() string {
	return strings.Join(od.Definition, "")
}

func isValidConnectionString(connectionDetails string) bool {
	if strings.Count(connectionDetails, ":") < 2 {
		return false
	}
	if strings.Count(connectionDetails, "@") < 1 {
		return false
	}
	if strings.Count(connectionDetails, "/") < 1 {
		return false
	}
	return true
}

func gitInitDirectory(gitAddress, filepath string) error {
	cmd := exec.Command("git", "clone", gitAddress, "./")
	cmd.Dir = filepath
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	cmd = exec.Command("git", "pull")
	cmd.Dir = filepath
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	return err
}

func gitCommitDirectory(filepath string) error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = filepath
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "commit", "-m", "sqlobjectexport updated SQL objects")
	cmd.Dir = filepath
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "push")
	cmd.Dir = filepath
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sqlobjectdump [--git] [--git-address] [--directory] [--schema=<schema>] [--type=<type>] <username>:<password>@<address>:<port>/<database>\n")
	}

	cfg := &config{}

	flag.BoolVar(&cfg.Git, "git", false, "initialize and/or commit git repository in directory")
	flag.StringVar(&cfg.GitAddress, "git-address", "", "git repository to operate on")
	flag.StringVar(&cfg.Directory, "directory", "", "root directory to export to")
	flag.StringVar(&cfg.DbSchema, "schema", "", "schema to export")
	flag.StringVar(&cfg.DbjectType, "type", "", "object type to export")

	flag.Parse()

	// expecting string of form <username>:<password>@<address>:<port>/<database>
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(-1)
	}

	if cfg.Directory == "" {
		flag.Usage()
		os.Exit(-1)
	}

	connectionDetails := args[0]
	if !isValidConnectionString(connectionDetails) {
		flag.Usage()
		os.Exit(-1)
	}

	cfg.DbUsername = connectionDetails[:strings.Index(connectionDetails, ":")]
	cfg.DbPassword = connectionDetails[strings.Index(connectionDetails, ":")+1 : strings.Index(connectionDetails, "@")]
	cfg.DbAddress = connectionDetails[strings.Index(connectionDetails, "@")+1 : strings.LastIndex(connectionDetails, ":")]
	cfg.DbPort = connectionDetails[strings.LastIndex(connectionDetails, ":")+1 : strings.LastIndex(connectionDetails, "/")]
	cfg.DbDatabase = connectionDetails[strings.Index(connectionDetails, "/")+1:]

	mssqldb, err := sql.Open("sqlserver", fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s;port=%s", cfg.DbAddress, cfg.DbUsername, cfg.DbPassword, cfg.DbDatabase, cfg.DbPort))
	if err != nil {
		log.Fatal(err)
	}

	var objects []*Object

	rows, err := mssqldb.Query(`SELECT
				s.[name] AS SchemaName,
				ao.[object_id] AS ObjectId, 
				ao.[name] AS ObjectName, 
				ao.[type] AS ObjectType, 
				ao.[type_desc] AS ObjectTypeDesc, 
				0 AS RetrievedDefinition
			FROM sys.all_objects ao
			JOIN sys.schemas s on s.schema_id = ao.schema_id
			WHERE 1=1
			AND ((@p1 = 0 AND ao.schema_id <> schema_id('sys')) OR (@p1 > 0 AND ao.schema_id = schema_id(@p2)))
			AND [type] NOT IN ('D', 'PK', 'SO', 'SQ', 'UQ', 'PC', 'FS', 'FT', 'F', 'SN', 'S', 'IT', 'AF')
			AND ((@p3 = 0) OR (@p3 > 0 AND [type] = @p4))`, len(cfg.DbSchema), cfg.DbSchema, len(cfg.DbjectType), cfg.DbjectType)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		obj := &Object{}
		err := rows.Scan(&obj.SchemaName, &obj.ObjectId, &obj.ObjectName, &obj.ObjectType, &obj.ObjectTypeDesc, &obj.RetrievedDefinition)
		if err != nil {
			log.Fatal(err)
		}
		objects = append(objects, obj)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Prepared %d objects for export", len(objects))

	for i, o := range objects {
		log.Printf("%d/%d Retrieving %s Definition for %s.%s [%s]", i+1, len(objects), o.ObjectTypeDesc, o.SchemaName, o.ObjectName, o.ObjectType)
		if strings.TrimSpace(o.ObjectType) == "U" {
			rows, err := mssqldb.Query(`SELECT 
						'[' + COLUMN_NAME + ']' + ' ' + '[' + DATA_TYPE + ']' + CASE WHEN CHARACTER_MAXIMUM_LENGTH != '' THEN '(' + CASE WHEN CAST(CHARACTER_MAXIMUM_LENGTH as varchar) = '-1' THEN 'MAX' ELSE CAST(CHARACTER_MAXIMUM_LENGTH as varchar) END + ') ' ELSE ' ' END + CASE WHEN IS_NULLABLE = 'YES' THEN 'NULL' ELSE 'NOT NULL' END AS Definition 
					FROM INFORMATION_SCHEMA.COLUMNS 
					WHERE TABLE_NAME = @p1 
					ORDER BY ORDINAL_POSITION ASC`, o.ObjectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			defer rows.Close()
			objects := &ObjectDefinition{Definition: []string{}}
			objects.Definition = append(objects.Definition, fmt.Sprintf("DECLARE @%s TABLE (\n", o.ObjectName))
			for rows.Next() {
				line := ""
				err := rows.Scan(&line)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v", err)
				}
				line = "\t" + line + ",\n"
				objects.Definition = append(objects.Definition, line)
			}
			objects.Definition[len(objects.Definition)-1] = strings.Replace(objects.Definition[len(objects.Definition)-1], ",", "", 1)
			objects.Definition = append(objects.Definition, ");")
			err = rows.Err()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			o.ObjectDefinition = o.generateCommentBlock() + objects.String()
		} else {
			rows, err := mssqldb.Query(`EXEC sp_helptext @p1`, fmt.Sprintf("%s.%s", o.SchemaName, o.ObjectName))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			defer rows.Close()
			objects := &ObjectDefinition{Definition: []string{}}
			for rows.Next() {
				line := ""
				err := rows.Scan(&line)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v", err)
				}
				objects.Definition = append(objects.Definition, line)
			}
			err = rows.Err()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
			}
			o.ObjectDefinition = o.generateCommentBlock() + objects.String()
		}
	}

	if cfg.Git {
		newpath := filepath.Join(cfg.Directory)
		os.MkdirAll(newpath, os.ModePerm)

		err = gitInitDirectory(cfg.GitAddress, cfg.Directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
		}
	}

	for _, o := range objects {
		newpath := filepath.Join(cfg.Directory, cfg.DbAddress, cfg.DbDatabase, o.SchemaName, o.ObjectTypeDesc)
		os.MkdirAll(newpath, os.ModePerm)

		objfilepath := filepath.Join(cfg.Directory, cfg.DbAddress, cfg.DbDatabase, o.SchemaName, o.ObjectTypeDesc, fmt.Sprintf("%s_%s.%s_%d.sql", o.SchemaName, o.ObjectName, o.ObjectType, o.ObjectId))
		d1 := []byte(o.ObjectDefinition)
		ioutil.WriteFile(objfilepath, d1, 0644)
	}

	if cfg.Git {
		err = gitCommitDirectory(cfg.Directory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
		}
	}

}

<h1 align="center">sqlobjexport</h1>
<p align="center">
<a href="https://godoc.org/github.com/cbergoon/sqlobjexport"><img src="https://img.shields.io/badge/godoc-reference-brightgreen.svg" alt="Docs"></a>
<a href="#"><img src="https://img.shields.io/badge/version-0.1.0-brightgreen.svg" alt="Version"></a>
</p>

`sqlobjexport` builds a file system representation of definitions for a specified set of SQL Server objects. 

#### Documentation 

See the docs [here](https://godoc.org/github.com/cbergoon/sqlobjexport).

#### Install
```
go get github.com/cbergoon/sqlobjexport
```

#### Example Usage

```
USAGE: Usage: sqlobjectdump [--git] [--git-address] [--directory] [--schema=<schema>] [--type=<type>] <username>:<password>@<address>:<port>/<database>
```

To retreive all stored procedures in the `dbo` schema in the `AdventureWorks` database: 

```
$ sqlobjexport -schema=dbo -type=P sa:password@127.0.0.1:1433/AdventureWorks
```

To use windows authentication:

```
$ sqlobjexport -schema=dbo -type=P 'domain\username:password@127.0.0.1:1433/AdventureWorks'
```

Available object types include: 

```
FN	SQL_SCALAR_FUNCTION
IF	SQL_INLINE_TABLE_VALUED_FUNCTION
V 	VIEW
AF	AGGREGATE_FUNCTION
S 	SYSTEM_TABLE
IT	INTERNAL_TABLE
P 	SQL_STORED_PROCEDURE
X 	EXTENDED_STORED_PROCEDURE
TF	SQL_TABLE_VALUED_FUNCTION
```

#### License
This project is licensed under the MIT License.









# Pending Features

## SQL Playground Enhancements

- Add a Ctrl + Shift + F shortcut that will format the SQL code in the editor in both the /sql full playground and the partial template embedded in challenges. By formatting I mean new lines, indents, etc, make it beautiful
- Add the Ctrl + Enter to run the entire block
- Add the Ctrl + Shift + Enter to run only the current statement (separated by ;, ending SQL Clause)
- Make it clearer to the user that when a challenge has the embedded SQL thing, the data will reside in a table named `dataset`. Also, make it so that the default SQL query that appears is something related to the dataset itself, or something like `SELECT * FROM dataset` if too complicated/not worth.
- Add some kind of scrolling to big results in the SQL results, instead of showing everything anyway. Something like a selectable button that by default LIMITs at 1k rows, but can be disabled for more rows.

# PROBLEMS.md Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix two bugs: (1) selected text invisible in dark mode, (2) per-challenge SQL Playground not displayed after being enabled at creation.

**Architecture:** Fix 1 adds `::selection` CSS to `custom.css` + links it from `base.html`. Fix 2 refactors `sql.html` into reusable Go template blocks (`sql-editor-html`, `sql-scripts`), then calls them from `challenge.html` inside a `{{if .Challenge.SQLEnabled}}` guard. A hidden `<div id="sql-challenge-meta">` element passes the dataset URL to JS via a `data-dataset-url` attribute, allowing `initDB()` to load a custom dataset instead of the CTF snapshot when provided.

**Tech Stack:** Go html/template, Tailwind CSS, HTMX, Alpine.js, DuckDB WASM, CodeMirror 6

---

### Task 1: Link custom.css in base.html and add ::selection rules

**Files:**
- Modify: `internal/views/templates/base.html:7`
- Modify: `internal/views/static/css/custom.css`

**Step 1: Add the `<link>` tag to base.html**

In `base.html`, after the Tailwind CDN script tag (line 7), add:

```html
<link rel="stylesheet" href="/static/css/custom.css">
```

The full head block should look like:
```html
<script src="https://cdn.tailwindcss.com?plugins=forms,typography,aspect-ratio,container-queries"></script>
<link rel="stylesheet" href="/static/css/custom.css">
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
```

**Step 2: Add `::selection` rules to custom.css**

Replace the current comment-only content of `internal/views/static/css/custom.css` with:

```css
/* Custom CSS for hCTF2 */

/* Text selection colors — browser defaults blend into dark background */
::selection {
    background-color: #7c3aed; /* purple-600 — matches UI accent */
    color: #ffffff;
}

.dark ::selection {
    background-color: #a855f7; /* purple-500 — slightly lighter for dark bg */
    color: #ffffff;
}
```

**Step 3: Rebuild and verify CSS loads**

Run:
```bash
task rebuild
./hctf2 --admin-email admin@test.com --admin-password password123 &
curl -s http://localhost:8080/static/css/custom.css
```

Expected: The `::selection` CSS content is returned (not a 404).

Kill the server: `pkill hctf2`

**Step 4: Commit**

```bash
git add internal/views/templates/base.html internal/views/static/css/custom.css
git commit -m "fix(ui): add text selection color for dark theme

Add ::selection rules using purple accent color so selected text is
clearly visible in both light and dark modes. Also links custom.css
in base.html (was embedded but never linked).

Closes PROBLEMS.md item 1"
```

---

### Task 2: Refactor sql.html into reusable template blocks

**Files:**
- Modify: `internal/views/templates/sql.html`

The current `sql.html` is one big `{{define "sql-content"}}` block. Split it into three named blocks:

- `{{define "sql-editor-html"}}` — the editor container div, CodeMirror CSS styles, Run/Clear buttons, results div
- `{{define "sql-scripts"}}` — the `<script type="importmap">` + `<script type="module">` blocks
- `{{define "sql-content"}}` — the page header + schema sidebar + calls to the two blocks above

**Step 1: Rewrite sql.html**

Replace the entire file with the following. Key change in `initDB`: check for a `<div id="sql-challenge-meta">` element with a `data-dataset-url` attribute. If found, load that URL as the dataset instead of the CTF snapshot.

```html
{{define "sql-editor-html"}}
<!-- CodeMirror editor container -->
<div id="editor-container" class="rounded border border-gray-200 dark:border-dark-border overflow-hidden sql-editor-container" style="min-height: 12rem;" tabindex="0"></div>

<!-- Theme-based CodeMirror styling -->
<style>
    html:not(.dark) .sql-editor-container .cm-editor {
        background-color: #ffffff !important;
    }
    html:not(.dark) .sql-editor-container .cm-content {
        background-color: #ffffff !important;
        color: #1f2937 !important;
    }
    html:not(.dark) .sql-editor-container .cm-gutters {
        background-color: #f3f4f6 !important;
        color: #6b7280 !important;
        border-right: 1px solid #e5e7eb !important;
    }
    html:not(.dark) .sql-editor-container .cm-activeLineGutter {
        background-color: #e5e7eb !important;
    }
    html:not(.dark) .sql-editor-container .cm-activeLine {
        background-color: #f3f4f6 !important;
    }
    html.dark .sql-editor-container .cm-editor {
        background-color: #0f172a !important;
    }
    html.dark .sql-editor-container .cm-content {
        background-color: #0f172a !important;
        color: #e2e8f0 !important;
    }
    html.dark .sql-editor-container .cm-gutters {
        background-color: #1e293b !important;
        color: #475569 !important;
        border-right: 1px solid #334155 !important;
    }
    html.dark .sql-editor-container .cm-activeLineGutter {
        background-color: #1e3a5f !important;
    }
    html.dark .sql-editor-container .cm-activeLine {
        background-color: #1e3a5f !important;
    }
</style>

<div class="flex gap-2 mt-4">
    <button onclick="window.executeQuery()"
            class="px-6 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded">
        Run Query
    </button>
    <button onclick="window.clearQuery()"
            class="px-6 py-2 bg-gray-500 dark:bg-gray-600 hover:bg-gray-600 dark:hover:bg-gray-700 text-white rounded">
        Clear
    </button>
</div>

<div id="query-results" class="bg-white dark:bg-dark-surface border border-gray-200 dark:border-dark-border rounded-lg p-4 min-h-64 mt-4">
    <p class="text-gray-500 dark:text-gray-400">Results will appear here...</p>
</div>
{{end}}


{{define "sql-scripts"}}
<!-- Import map to resolve module dependencies -->
<script type="importmap">
{
    "imports": {
        "apache-arrow": "https://cdn.jsdelivr.net/npm/apache-arrow@13.0.0/+esm"
    }
}
</script>

<script type="module">
    import * as duckdb from 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@latest/dist/duckdb-browser.mjs';
    import * as arrow from 'apache-arrow';
    import {EditorView, basicSetup} from 'https://esm.sh/codemirror';
    import {sql} from 'https://esm.sh/@codemirror/lang-sql';

    let db, conn;
    let duckdbReady = false;
    let editor;

    const startDoc = `SELECT c.name, c.category, c.difficulty, COUNT(s.id) as solves
FROM challenges c
LEFT JOIN questions q ON c.id = q.challenge_id
LEFT JOIN submissions s ON q.id = s.question_id AND s.is_correct = 1
WHERE c.visible = 1
GROUP BY c.id, c.name, c.category, c.difficulty
ORDER BY solves DESC;`;

    const isDarkTheme = document.documentElement.classList.contains('dark');

    const editorTheme = EditorView.theme({
        '&': { fontSize: '0.875rem', height: '12rem' },
        '.cm-scroller': { overflow: 'auto', fontFamily: "'Courier New', Courier, monospace" },
        '&.cm-focused': { outline: 'none' },
    });

    const darkTheme = EditorView.theme({
        '&': { backgroundColor: '#0f172a' },
        '.cm-content': { backgroundColor: '#0f172a', color: '#e2e8f0', caretColor: '#60a5fa' },
        '.cm-cursor': { borderLeftColor: '#60a5fa' },
        '.cm-gutters': { backgroundColor: '#1e293b', color: '#475569', borderRight: '1px solid #334155' },
        '.cm-activeLineGutter': { backgroundColor: '#1e3a5f' },
        '.cm-activeLine': { backgroundColor: '#1e3a5f' },
        '.cm-selectionBackground': { backgroundColor: '#2d4a7a' },
    });

    const lightTheme = EditorView.theme({
        '&': { backgroundColor: '#ffffff' },
        '.cm-content': { backgroundColor: '#ffffff', color: '#1f2937', caretColor: '#2563eb' },
        '.cm-cursor': { borderLeftColor: '#2563eb' },
        '.cm-gutters': { backgroundColor: '#f3f4f6', color: '#6b7280', borderRight: '1px solid #e5e7eb' },
        '.cm-activeLineGutter': { backgroundColor: '#e5e7eb' },
        '.cm-activeLine': { backgroundColor: '#f3f4f6' },
        '.cm-selectionBackground': { backgroundColor: '#bfdbfe' },
    });

    async function initEditor() {
        if (isDarkTheme) {
            const {oneDark} = await import('https://esm.sh/@codemirror/theme-one-dark');
            editor = new EditorView({
                doc: startDoc,
                extensions: [basicSetup, sql(), oneDark, darkTheme, editorTheme, EditorView.lineWrapping],
                parent: document.getElementById('editor-container'),
            });
        } else {
            editor = new EditorView({
                doc: startDoc,
                extensions: [basicSetup, sql(), lightTheme, editorTheme, EditorView.lineWrapping],
                parent: document.getElementById('editor-container'),
            });
        }
        window.editor = editor;
    }

    initEditor();

    const themeObserver = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
            if (mutation.attributeName === 'class') {
                const newIsDark = document.documentElement.classList.contains('dark');
                if (newIsDark !== isDarkTheme) {
                    window.location.reload();
                }
            }
        });
    });
    themeObserver.observe(document.documentElement, { attributes: true });

    const exampleQueries = [
        `-- Top 10 users by points
SELECT u.name, SUM(q.points) as total_points, COUNT(*) as solves
FROM users u
JOIN submissions s ON u.id = s.user_id
JOIN questions q ON s.question_id = q.id
WHERE s.is_correct = 1
GROUP BY u.id, u.name
ORDER BY total_points DESC
LIMIT 10;`,

        `-- Most difficult challenges (fewest solves)
SELECT c.name, c.category, c.difficulty, COUNT(s.id) as solve_count
FROM challenges c
LEFT JOIN questions q ON c.id = q.challenge_id
LEFT JOIN submissions s ON q.id = s.question_id AND s.is_correct = 1
GROUP BY c.id, c.name, c.category, c.difficulty
ORDER BY solve_count ASC
LIMIT 10;`,

        `-- Category statistics
SELECT category, COUNT(*) as challenge_count, AVG(solve_count) as avg_solves
FROM (
    SELECT c.category, c.id, COUNT(s.id) as solve_count
    FROM challenges c
    LEFT JOIN questions q ON c.id = q.challenge_id
    LEFT JOIN submissions s ON q.id = s.question_id AND s.is_correct = 1
    GROUP BY c.category, c.id
)
GROUP BY category;`,

        `-- Team leaderboard
SELECT t.name as team, COUNT(DISTINCT s.question_id) as solves, SUM(q.points) as total_points
FROM teams t
JOIN users u ON u.team_id = t.id
JOIN submissions s ON s.user_id = u.id AND s.is_correct = 1
JOIN questions q ON q.id = s.question_id
GROUP BY t.id, t.name
ORDER BY total_points DESC;`,
    ];

    window.exampleQueries = exampleQueries;

    window.setQuery = function(query) {
        if (!window.editor) return;
        window.editor.dispatch({
            changes: { from: 0, to: window.editor.state.doc.length, insert: query }
        });
        window.editor.focus();
    };

    window.clearQuery = function() {
        if (!window.editor) return;
        window.editor.dispatch({
            changes: { from: 0, to: window.editor.state.doc.length, insert: '' }
        });
        document.getElementById('query-results').innerHTML = '<p class="text-gray-500 dark:text-gray-400">Results will appear here...</p>';
        window.editor.focus();
    };

    window.executeQuery = async function() {
        if (!window.editor) return;
        const query = window.editor.state.doc.toString().trim();
        if (!query) return;

        const resultsDiv = document.getElementById('query-results');

        if (!duckdbReady || !db || !conn) {
            resultsDiv.innerHTML = '<p class="text-yellow-600 dark:text-yellow-400">⚠️ Database is still loading. Please wait...</p>';
            return;
        }

        resultsDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Executing query...</p>';

        try {
            const result = await conn.query(query);
            const rows = result.toArray();

            if (rows.length === 0) {
                resultsDiv.innerHTML = '<p class="text-yellow-600 dark:text-yellow-400">No results found</p>';
                return;
            }

            const columns = Object.keys(rows[0]);
            let html = '<div class="overflow-x-auto"><table class="w-full text-sm"><thead class="bg-gray-100 dark:bg-dark-bg"><tr>';

            columns.forEach(col => {
                html += `<th class="px-4 py-2 text-left text-gray-600 dark:text-gray-400">${col}</th>`;
            });
            html += '</tr></thead><tbody class="divide-y divide-gray-200 dark:divide-dark-border">';

            rows.forEach(row => {
                html += '<tr class="hover:bg-gray-100 dark:hover:bg-dark-bg">';
                columns.forEach(col => {
                    const value = row[col] === null ? '<span class="text-gray-400">NULL</span>' : row[col];
                    html += `<td class="px-4 py-2 text-gray-700 dark:text-gray-300">${value}</td>`;
                });
                html += '</tr>';
            });

            html += '</tbody></table></div>';
            html += `<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">${rows.length} row(s) returned</p>`;
            resultsDiv.innerHTML = html;
        } catch (error) {
            resultsDiv.innerHTML = `<p class="text-red-600 dark:text-red-400">Error: ${error.message}</p>`;
        }
    };

    async function initDB() {
        const resultsDiv = document.getElementById('query-results');
        resultsDiv.innerHTML = '<p class="text-gray-500 dark:text-gray-400">Loading DuckDB WASM...</p>';

        try {
            const DUCKDB_CONFIG = await duckdb.selectBundle(duckdb.getJsDelivrBundles());
            const logger = new duckdb.ConsoleLogger();
            const worker = await duckdb.createWorker(DUCKDB_CONFIG.mainWorker);
            db = new duckdb.AsyncDuckDB(logger, worker);
            await db.instantiate(DUCKDB_CONFIG.mainModule, DUCKDB_CONFIG.pthreadWorker);
            conn = await db.connect();

            // Check if a per-challenge dataset URL is provided
            const metaEl = document.getElementById('sql-challenge-meta');
            const customDatasetURL = metaEl ? metaEl.getAttribute('data-dataset-url') : null;

            if (customDatasetURL) {
                // Load the challenge-specific dataset from the provided URL
                await conn.query(`CREATE TABLE dataset AS SELECT * FROM '${customDatasetURL}'`);
                duckdbReady = true;
                resultsDiv.innerHTML = '<p class="text-green-600 dark:text-green-400">✅ Dataset loaded! Run a query to get started.</p>';
            } else {
                // Load from CTF snapshot (global /sql page)
                const snapshot = await fetch('/api/sql/snapshot').then(r => r.json());

                if (snapshot.challenges && snapshot.challenges.length > 0) {
                    const records = snapshot.challenges.map(c => ({
                        id: c.id, name: c.name, category: c.category,
                        difficulty: c.difficulty, visible: c.visible ? 1 : 0,
                    }));
                    await conn.insertArrowTable(arrow.tableFromJSON(records), { name: 'challenges' });
                }

                if (snapshot.questions && snapshot.questions.length > 0) {
                    const records = snapshot.questions.map(q => ({
                        id: q.id, challenge_id: q.challenge_id, name: q.name,
                        description: q.description || '', flag_mask: q.flag_mask || '',
                        points: q.points, created_at: q.created_at || '',
                    }));
                    await conn.insertArrowTable(arrow.tableFromJSON(records), { name: 'questions' });
                }

                if (snapshot.submissions && snapshot.submissions.length > 0) {
                    const records = snapshot.submissions.map(s => ({
                        id: s.id, question_id: s.question_id, user_id: s.user_id,
                        team_id: s.team_id || null, user_name: s.user_name || '',
                        is_correct: s.is_correct ? 1 : 0, created_at: s.created_at || '',
                    }));
                    await conn.insertArrowTable(arrow.tableFromJSON(records), { name: 'submissions' });
                }

                if (snapshot.users && snapshot.users.length > 0) {
                    const records = snapshot.users.map(u => ({
                        id: u.id, name: u.name, team_id: u.team_id || null,
                        created_at: u.created_at || '',
                    }));
                    await conn.insertArrowTable(arrow.tableFromJSON(records), { name: 'users' });
                }

                if (snapshot.teams && snapshot.teams.length > 0) {
                    const records = snapshot.teams.map(t => ({
                        id: t.id, name: t.name, description: t.description || '',
                        owner_id: t.owner_id, created_at: t.created_at || '',
                    }));
                    await conn.insertArrowTable(arrow.tableFromJSON(records), { name: 'teams' });
                }

                duckdbReady = true;
                resultsDiv.innerHTML = '<p class="text-green-600 dark:text-green-400">✅ Database loaded! Run a query to get started.</p>';
            }
        } catch (error) {
            duckdbReady = false;
            console.error('DuckDB initialization error:', error);
            resultsDiv.innerHTML = `<p class="text-red-600 dark:text-red-400">Failed to initialize DuckDB</p><p class="text-gray-500 dark:text-gray-400 text-sm mt-2">Error: ${error.message}</p>`;
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initDB);
    } else {
        initDB();
    }
</script>
{{end}}


{{define "sql-content"}}
<div class="mb-8">
    <h1 class="text-4xl font-bold text-blue-400 mb-4">SQL Playground</h1>
    <p class="text-gray-600 dark:text-gray-300">Query CTF data using SQL (powered by DuckDB WASM)</p>
</div>

<div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <!-- Schema sidebar -->
    <div class="lg:col-span-1 bg-white dark:bg-dark-surface border border-gray-200 dark:border-dark-border rounded-lg p-4 flex flex-col">
        <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-3 shrink-0">Schema</h3>
        <div class="overflow-y-auto flex-1 space-y-4 text-xs pr-1" style="max-height: 520px;">

            <div>
                <h4 class="font-bold text-blue-400 mb-1 uppercase tracking-wide">challenges</h4>
                <table class="w-full text-gray-600 dark:text-gray-400">
                    <tbody>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">name</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">category</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">difficulty</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">visible</td><td class="text-gray-500 dark:text-gray-600">INTEGER</td></tr>
                    </tbody>
                </table>
            </div>

            <div>
                <h4 class="font-bold text-blue-400 mb-1 uppercase tracking-wide">questions</h4>
                <table class="w-full text-gray-600 dark:text-gray-400">
                    <tbody>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">challenge_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">name</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">description</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">flag_mask</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">points</td><td class="text-gray-500 dark:text-gray-600">INTEGER</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">created_at</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                    </tbody>
                </table>
            </div>

            <div>
                <h4 class="font-bold text-blue-400 mb-1 uppercase tracking-wide">submissions <span class="text-gray-500 dark:text-gray-600 normal-case font-normal">(correct only)</span></h4>
                <table class="w-full text-gray-600 dark:text-gray-400">
                    <tbody>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">question_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">user_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">team_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">user_name</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">is_correct</td><td class="text-gray-500 dark:text-gray-600">INTEGER</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">created_at</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                    </tbody>
                </table>
            </div>

            <div>
                <h4 class="font-bold text-blue-400 mb-1 uppercase tracking-wide">users</h4>
                <table class="w-full text-gray-600 dark:text-gray-400">
                    <tbody>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">name</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">team_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">created_at</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                    </tbody>
                </table>
            </div>

            <div>
                <h4 class="font-bold text-blue-400 mb-1 uppercase tracking-wide">teams</h4>
                <table class="w-full text-gray-600 dark:text-gray-400">
                    <tbody>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">name</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">description</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">owner_id</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                        <tr><td class="font-mono pr-2 py-0.5 text-gray-700 dark:text-gray-300">created_at</td><td class="text-gray-500 dark:text-gray-600">TEXT</td></tr>
                    </tbody>
                </table>
            </div>

        </div>

        <div class="mt-4 pt-4 border-t border-gray-200 dark:border-dark-border shrink-0">
            <h4 class="font-bold text-gray-900 dark:text-white mb-2 text-sm">Examples</h4>
            <button onclick="window.setQuery(window.exampleQueries[0])" class="text-left w-full text-xs text-blue-500 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 mb-2 leading-tight">
                Top 10 users by score
            </button>
            <button onclick="window.setQuery(window.exampleQueries[1])" class="text-left w-full text-xs text-blue-500 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 mb-2 leading-tight">
                Hardest challenges
            </button>
            <button onclick="window.setQuery(window.exampleQueries[2])" class="text-left w-full text-xs text-blue-500 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 mb-2 leading-tight">
                Category stats
            </button>
            <button onclick="window.setQuery(window.exampleQueries[3])" class="text-left w-full text-xs text-blue-500 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 leading-tight">
                Team leaderboard
            </button>
        </div>
    </div>

    <!-- Query editor and results -->
    <div class="lg:col-span-3 space-y-4">
        {{template "sql-editor-html" .}}
    </div>
</div>

{{template "sql-scripts" .}}
{{end}}
```

**Step 2: Rebuild and verify the global /sql page still works**

```bash
task rebuild
./hctf2 --admin-email admin@test.com --admin-password password123 &
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/sql
```

Expected: `200`

Kill the server: `pkill hctf2`

**Step 3: Commit**

```bash
git add internal/views/templates/sql.html
git commit -m "refactor(sql): extract editor into reusable template blocks

Split sql-content into sql-editor-html and sql-scripts blocks so
the DuckDB editor can be embedded in other pages. initDB now checks
for a sql-challenge-meta element with data-dataset-url attribute to
optionally load a custom dataset for per-challenge SQL playgrounds."
```

---

### Task 3: Add SQL Playground section to challenge.html

**Files:**
- Modify: `internal/views/templates/challenge.html`

**Step 1: Add the SQL section after the questions loop**

After line 89 (`</div>` that closes the questions loop, before `{{end}}`), add:

```html
{{if .Challenge.SQLEnabled}}
{{/* Hidden metadata element — passes dataset URL to initDB() via data attribute */}}
<div id="sql-challenge-meta" class="hidden"
     {{if .Challenge.SQLDatasetURL}}data-dataset-url="{{.Challenge.SQLDatasetURL}}"{{end}}></div>

<section class="mt-8 bg-white dark:bg-dark-surface border border-gray-200 dark:border-dark-border rounded-lg p-6">
    <h2 class="text-2xl font-bold text-blue-400 mb-2">SQL Playground</h2>
    <p class="text-gray-500 dark:text-gray-400 text-sm mb-4">
        Analyze data with SQL (powered by DuckDB WASM — runs in your browser)
    </p>

    {{if .Challenge.SQLSchemaHint}}
    <div class="mb-6">
        <h3 class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-2">Schema Reference</h3>
        <pre class="bg-gray-100 dark:bg-dark-bg border border-gray-200 dark:border-dark-border p-4 rounded text-sm overflow-x-auto font-mono text-gray-900 dark:text-gray-100">{{.Challenge.SQLSchemaHint}}</pre>
    </div>
    {{end}}

    {{template "sql-editor-html" .}}
</section>

{{template "sql-scripts" .}}
{{end}}
```

The final `challenge.html` should end:
```
    {{end}}
</div>

{{if .Challenge.SQLEnabled}}
... (new section above) ...
{{end}}
{{end}}
```

**Step 2: Rebuild and verify the build succeeds**

```bash
task rebuild
```

Expected: Build completes without errors. If there are template errors, they appear at startup when the server renders its first page.

```bash
./hctf2 --admin-email admin@test.com --admin-password password123 &
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/challenges
```

Expected: `200` (challenges page loads)

Kill the server: `pkill hctf2`

**Step 3: Commit**

```bash
git add internal/views/templates/challenge.html
git commit -m "feat(challenge): display SQL Playground when enabled per-challenge

Add conditional SQL Playground section to challenge detail page.
When a challenge has sql_enabled=true, renders the DuckDB editor
below the questions. Shows sql_schema_hint as a code block reference.
If sql_dataset_url is set, loads that URL instead of CTF snapshot.

Closes PROBLEMS.md item 2"
```

---

### Task 4: Validate with agent-browser

**Step 1: Rebuild and start server**

```bash
task rebuild
./hctf2 --admin-email admin@test.com --admin-password password123 &
sleep 2
```

**Step 2: Open browser in headed mode**

```bash
npx agent-browser --headed --session hctf2 open http://localhost:8080
```

**Step 3: Validate Fix 1 — text selection in dark mode**

1. Navigate to `/login`
2. Click the email input field
3. Type some text, then select it (Ctrl+A or click-drag)
4. Verify the selection highlight is purple and clearly visible (not dark/invisible)
5. Click the theme toggle to switch to light mode
6. Select text again — verify it is also clearly visible in light mode

Take a screenshot: `npx agent-browser --session hctf2 screenshot --full fix1-selection.png`

**Step 4: Create a test challenge with SQL enabled (via admin)**

1. Navigate to `/admin`
2. Login as admin if prompted
3. Create a challenge with:
   - Name: "SQL Test Challenge"
   - Description: "Testing SQL playground"
   - SQL Playground: enabled (check the checkbox)
   - Schema Hint: `-- Table: employees (id, name, salary, department)`
4. Note the challenge ID from the URL after creation

**Step 5: Validate Fix 2 — SQL playground appears on challenge page**

1. Navigate to the challenge page for the newly created challenge
2. Scroll past the questions section
3. Verify a "SQL Playground" section appears at the bottom
4. Verify the schema hint is displayed as a code block
5. Verify the CodeMirror editor loads (may take a few seconds for DuckDB WASM)
6. Run a simple query: `SELECT 1 + 1 AS result;`
7. Verify results appear in the results pane

**Step 6: Validate that challenges WITHOUT SQL show no playground**

1. Create a second challenge WITHOUT enabling SQL
2. Navigate to that challenge page
3. Verify no SQL Playground section appears

**Step 7: Validate global /sql page still works**

1. Navigate to `/sql`
2. Verify the full page with schema sidebar renders
3. Run `SELECT 1 AS test;` and verify results appear

Take a final screenshot: `npx agent-browser --session hctf2 screenshot --full fix2-sql-playground.png`

**Step 8: Kill server**

```bash
pkill hctf2
```

---

### Task 5: Final commit

After all validation passes:

```bash
git add -A
git status
```

If everything looks clean (only expected files changed):

```bash
git log --oneline -5
```

Verify the three commits from Tasks 1-3 are present and meaningful.

No further commit needed if Tasks 1-3 were committed individually.

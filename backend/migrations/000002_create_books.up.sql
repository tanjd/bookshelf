CREATE TABLE books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    isbn TEXT,
    ol_key TEXT UNIQUE,
    cover_url TEXT,
    description TEXT
);

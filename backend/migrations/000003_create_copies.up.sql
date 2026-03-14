CREATE TABLE copies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id),
    owner_id INTEGER NOT NULL REFERENCES users(id),
    condition TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'available'
);

-- Test data for integration tests
CREATE TABLE IF NOT EXISTS customers (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    country TEXT NOT NULL,
    city TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id),
    amount DECIMAL(10, 2) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now()
);

INSERT INTO customers (name, country, city) VALUES
    ('Alice', 'USA', 'New York'),
    ('Bob', 'UK', 'London'),
    ('Charlie', 'USA', 'Chicago'),
    ('Diana', 'Germany', 'Berlin');

INSERT INTO orders (customer_id, amount, status) VALUES
    (1, 100.00, 'completed'),
    (1, 200.00, 'completed'),
    (2, 150.00, 'completed'),
    (3, 300.00, 'pending'),
    (4, 50.00, 'completed');

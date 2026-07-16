INSERT INTO users (
    email,
    password_hash,
    name,
    role
)
VALUES (
    'admin@example.com',
    crypt('Admin123!', gen_salt('bf', 12)),
    'Admin',
    'admin'
)
ON CONFLICT (email) DO NOTHING;

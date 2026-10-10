SELECT 'CREATE DATABASE edvance_auth OWNER postgres'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_auth')\gexec;

SELECT 'CREATE DATABASE edvance_admin OWNER postgres'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_admin')\gexec;

SELECT 'CREATE DATABASE edvance_analytics OWNER postgres'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_analytics')\gexec;

SELECT 'CREATE DATABASE edvance_moderation OWNER postgres'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_moderation')\gexec;

SELECT 'CREATE DATABASE edvance_recommendation OWNER postgres'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_recommendation')\gexec;

SELECT 'CREATE DATABASE edvance_auth'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_auth')\gexec

SELECT 'CREATE DATABASE edvance_admin'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_admin')\gexec

SELECT 'CREATE DATABASE edvance_analytics'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_analytics')\gexec

SELECT 'CREATE DATABASE edvance_moderation'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_moderation')\gexec

SELECT 'CREATE DATABASE edvance_recommendation'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_recommendation')\gexec

SELECT 'CREATE DATABASE edvance_course'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_course')\gexec

SELECT 'CREATE DATABASE edvance_learning'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_learning')\gexec

SELECT 'CREATE DATABASE edvance_commerce'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_commerce')\gexec

SELECT 'CREATE DATABASE edvance_payment'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'edvance_payment')\gexec

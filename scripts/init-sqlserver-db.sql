-- Runs once via sqlserver-init after test-sqlserver is healthy.
IF DB_ID(N'test_data') IS NULL
    CREATE DATABASE test_data;
GO

IF NOT EXISTS (SELECT 1 FROM sys.server_principals WHERE name = N'test_user')
    CREATE LOGIN test_user WITH PASSWORD = N'Test_password123!', CHECK_POLICY = OFF;
GO

USE test_data;
GO

IF NOT EXISTS (SELECT 1 FROM sys.database_principals WHERE name = N'test_user')
BEGIN
    CREATE USER test_user FOR LOGIN test_user;
    ALTER ROLE db_owner ADD MEMBER test_user;
END
GO

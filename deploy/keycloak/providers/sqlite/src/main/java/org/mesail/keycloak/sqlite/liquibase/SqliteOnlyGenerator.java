package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.exception.ValidationErrors;
import liquibase.sqlgenerator.SqlGenerator;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.sqlgenerator.core.AbstractSqlGenerator;
import liquibase.statement.SqlStatement;

public abstract class SqliteOnlyGenerator<T extends SqlStatement> extends AbstractSqlGenerator<T> {

    private static final int PRIORITY = SqlGenerator.PRIORITY_DATABASE + 1;

    @Override
    public int getPriority() {
        return PRIORITY;
    }

    @Override
    public boolean supports(T statement, Database database) {
        return database instanceof SQLiteDatabase;
    }

    @Override
    public ValidationErrors validate(T statement, Database database, SqlGeneratorChain<T> chain) {
        return new ValidationErrors();
    }
}

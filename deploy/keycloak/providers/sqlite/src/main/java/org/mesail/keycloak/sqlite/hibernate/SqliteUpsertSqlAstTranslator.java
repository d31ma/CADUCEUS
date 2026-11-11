package org.mesail.keycloak.sqlite.hibernate;

import org.hibernate.community.dialect.SQLiteSqlAstTranslator;
import org.hibernate.engine.spi.SessionFactoryImplementor;
import org.hibernate.sql.ast.tree.Statement;
import org.hibernate.sql.ast.tree.insert.ConflictClause;
import org.hibernate.sql.exec.spi.JdbcOperation;

public class SqliteUpsertSqlAstTranslator<T extends JdbcOperation> extends SQLiteSqlAstTranslator<T> {

    public SqliteUpsertSqlAstTranslator(SessionFactoryImplementor factory, Statement statement) {
        super(factory, statement);
    }

    @Override
    protected void visitConflictClause(ConflictClause conflictClause) {
        visitPresentConflictClause(conflictClause);
    }

    private void visitPresentConflictClause(ConflictClause conflictClause) {
        if (conflictClause == null) {
            return;
        }
        visitStandardConflictClause(conflictClause);
    }
}

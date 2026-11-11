package org.mesail.keycloak.sqlite.hibernate;

import org.hibernate.engine.spi.SessionFactoryImplementor;
import org.hibernate.sql.ast.SqlAstTranslator;
import org.hibernate.sql.ast.spi.StandardSqlAstTranslatorFactory;
import org.hibernate.sql.ast.tree.Statement;
import org.hibernate.sql.exec.spi.JdbcOperation;

public class SqliteUpsertTranslatorFactory extends StandardSqlAstTranslatorFactory {

    @Override
    protected <T extends JdbcOperation> SqlAstTranslator<T> buildTranslator(SessionFactoryImplementor factory, Statement statement) {
        return new SqliteUpsertSqlAstTranslator<>(factory, statement);
    }
}

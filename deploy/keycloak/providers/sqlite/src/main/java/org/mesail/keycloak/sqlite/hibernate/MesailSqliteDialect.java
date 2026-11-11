package org.mesail.keycloak.sqlite.hibernate;

import org.hibernate.community.dialect.SQLiteDialect;
import org.hibernate.engine.jdbc.dialect.spi.DialectResolutionInfo;
import org.hibernate.sql.ast.SqlAstTranslatorFactory;

public class MesailSqliteDialect extends SQLiteDialect {

    public MesailSqliteDialect() {
        super();
    }

    public MesailSqliteDialect(DialectResolutionInfo info) {
        super(info);
    }

    @Override
    public SqlAstTranslatorFactory getSqlAstTranslatorFactory() {
        return new SqliteUpsertTranslatorFactory();
    }
}

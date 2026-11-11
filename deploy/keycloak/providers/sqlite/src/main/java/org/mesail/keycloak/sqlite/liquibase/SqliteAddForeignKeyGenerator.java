package org.mesail.keycloak.sqlite.liquibase;

import java.util.ArrayList;
import java.util.List;
import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.AddForeignKeyConstraintStatement;
import liquibase.structure.core.Column;
import liquibase.structure.core.Table;
import liquibase.util.StringUtil;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteAddForeignKeyGenerator extends SqliteOnlyGenerator<AddForeignKeyConstraintStatement> {

    private static final String CASCADE = "CASCADE";
    private static final String COLUMN_SEPARATOR = ",";

    @Override
    public Sql[] generateSql(AddForeignKeyConstraintStatement statement, Database database, SqlGeneratorChain<AddForeignKeyConstraintStatement> chain) {
        return cascadeTriggerOrNothing(statement, database);
    }

    private Sql[] cascadeTriggerOrNothing(AddForeignKeyConstraintStatement statement, Database database) {
        if (!CASCADE.equalsIgnoreCase(statement.getOnDelete())) {
            return SqliteSql.none();
        }
        return SqliteSql.single(cascadeTrigger(statement, database));
    }

    private String cascadeTrigger(AddForeignKeyConstraintStatement statement, Database database) {
        return "CREATE TRIGGER IF NOT EXISTS " + database.escapeObjectName(statement.getConstraintName(), Table.class)
                + " AFTER DELETE ON " + database.escapeObjectName(statement.getReferencedTableName(), Table.class)
                + " FOR EACH ROW BEGIN DELETE FROM " + database.escapeObjectName(statement.getBaseTableName(), Table.class)
                + " WHERE " + matchCondition(statement, database) + "; END";
    }

    private String matchCondition(AddForeignKeyConstraintStatement statement, Database database) {
        List<String> baseColumns = StringUtil.splitAndTrim(statement.getBaseColumnNames(), COLUMN_SEPARATOR);
        List<String> referencedColumns = StringUtil.splitAndTrim(statement.getReferencedColumnNames(), COLUMN_SEPARATOR);
        List<String> conditions = new ArrayList<>();
        for (int index = 0; index < baseColumns.size(); index++) {
            conditions.add(database.escapeObjectName(baseColumns.get(index), Column.class) + " = OLD."
                    + database.escapeObjectName(referencedColumns.get(index), Column.class));
        }
        return String.join(" AND ", conditions);
    }
}

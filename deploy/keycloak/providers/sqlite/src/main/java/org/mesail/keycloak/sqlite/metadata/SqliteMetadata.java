package org.mesail.keycloak.sqlite.metadata;

import java.lang.reflect.InvocationHandler;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.sql.Connection;
import java.sql.DatabaseMetaData;

public class SqliteMetadata implements InvocationHandler {

    private static final String GET_COLUMNS = "getColumns";
    private static final String MATCH_ALL = "%";
    private static final int TABLE_PATTERN_INDEX = 2;
    private static final int COLUMN_PATTERN_INDEX = 3;

    private final DatabaseMetaData metadata;
    private final SqliteColumnTable columnTable;

    private SqliteMetadata(DatabaseMetaData metadata, Connection connection) {
        this.metadata = metadata;
        this.columnTable = new SqliteColumnTable(metadata, connection);
    }

    public static DatabaseMetaData wrap(DatabaseMetaData metadata, Connection connection) {
        return (DatabaseMetaData) Proxy.newProxyInstance(DatabaseMetaData.class.getClassLoader(), new Class<?>[] {DatabaseMetaData.class},
                new SqliteMetadata(metadata, connection));
    }

    @Override
    public Object invoke(Object proxy, Method method, Object[] arguments) throws Throwable {
        return answer(method, arguments);
    }

    private Object answer(Method method, Object[] arguments) throws Throwable {
        if (isColumnsOfAllTables(method, arguments)) {
            return columnTable.columnsOfAllTables((String) arguments[0], (String) arguments[1], (String) arguments[COLUMN_PATTERN_INDEX]);
        }
        return forward(method, arguments);
    }

    private boolean isColumnsOfAllTables(Method method, Object[] arguments) {
        return GET_COLUMNS.equals(method.getName()) && isMatchAll((String) arguments[TABLE_PATTERN_INDEX]);
    }

    private boolean isMatchAll(String tablePattern) {
        return tablePattern == null || MATCH_ALL.equals(tablePattern);
    }

    private Object forward(Method method, Object[] arguments) throws Throwable {
        try {
            return method.invoke(metadata, arguments);
        } catch (InvocationTargetException failure) {
            throw failure.getCause();
        }
    }
}

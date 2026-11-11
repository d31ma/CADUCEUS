package org.mesail.keycloak.sqlite.metadata;

import java.lang.reflect.InvocationHandler;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.sql.Connection;
import java.sql.DatabaseMetaData;

public class SqliteMetadataConnection implements InvocationHandler {

    private static final String GET_METADATA = "getMetaData";

    private final Connection connection;

    private SqliteMetadataConnection(Connection connection) {
        this.connection = connection;
    }

    public static Connection wrap(Connection connection) {
        return (Connection) Proxy.newProxyInstance(Connection.class.getClassLoader(), new Class<?>[] {Connection.class},
                new SqliteMetadataConnection(connection));
    }

    @Override
    public Object invoke(Object proxy, Method method, Object[] arguments) throws Throwable {
        Object result = forward(method, arguments);
        return wrapMetadata(method, result);
    }

    private Object forward(Method method, Object[] arguments) throws Throwable {
        try {
            return method.invoke(connection, arguments);
        } catch (InvocationTargetException failure) {
            throw failure.getCause();
        }
    }

    private Object wrapMetadata(Method method, Object result) {
        if (!GET_METADATA.equals(method.getName())) {
            return result;
        }
        return SqliteMetadata.wrap((DatabaseMetaData) result, connection);
    }
}

-- Открываем порт для доступа по iproto
box.cfg({listen="127.0.0.1:3301"})
-- Создаём пользователя для подключения
box.schema.user.create('storage', {password='passw0rd', if_not_exists=true})
-- Даём все-все права
box.schema.user.grant('storage', 'super', nil, nil, {if_not_exists=true})

-- создаём таблицу для хранения отзывов на карте
box.schema.space.create('cats', {if_not_exists=true})
box.space.cats:format({
        {name="id", type="string"},
        {name="coordinates", type="array"},
        {name="name", type="string"}
})
-- создаём первичный индекс
box.space.cats:create_index('primary', {
                                parts={{ field="id", type="string" }},
                                type = 'TREE',
                                if_not_exists=true,})
-- создаём индекс для координат
box.space.cats:create_index('spatial', {
                                parts = {{ field="coordinates", type='array'} },
                                type = 'RTREE',
                                unique = false,
                                if_not_exists=true,})

require('console').start() os.exit()
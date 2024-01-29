в этом задании мы пишем аналог unix pipeline, что-то вроде:

cat common.go | grep func | sort | uniq -c | sort -n  

когда STDOUT одной программы передается как STDIN в другую программу
но в нашем случае эти роли выполняют каналы, которые мы передаем из одной функции в другую.
само задание по сути состоит из двух частей

написание функции RunPipeline которая обеспечивает нам конвейерную обработку функций-воркеров, которые что-то делают.
написание нескольких функций, которые считают нам информацию по письмам пользователей

если кратко, то мы хотим запустить следующую цепочку:

cat emails.txt | SelectUsers | SelectMessages | CheckSpam | CombineResults

то есть мы получаем на вход имейлы юзеров, селектим их из "базы" и получаем каждому юзеру user_id. затем селектим по каждому user_id список писем(msg_id) этого юзера. дальше проверяем эти письма на спам и выдаем итоговый результат: какие у письма со спамом, а какие нет.
а теперь подробнее про реализацию этой цепочки:

SelectUsers()

in - string внутри interface{}. это имейлы юзеров
out - отдает структурки User{}. это результат функции GetUser()
особенности:
GetUser() выполняется 1 секунду. его можно вызывать параллельно для нескольких юзеров. это экономит время
у некоторых юзеров есть alias'ы(псевдонимы). то есть на вид это два разных имейла, но на самом деле это один и тот же юзер в базе. например "batman@mail.ru" - это алиас к "bruce.wayne@mail.ru". SelectUsers() должен отдавать в out только уникальных юзеров.


SelectMessages()

in -  User{} внутри interface{} от SelectUsers()
out - отдает MsgID. это айдишники писем юзеров - результат функции GetMessages()
особенности:
GetMessages() выполняется 1 секунду. его тоже можно вызывать параллельно.
но GetMessages() позволяет использовать "батчи". то есть за раз в нее можно запихнуть не 1 юзера, а несколько. максимальное кол-во юзеров = 2. то есть если мы хотим селектнуть у 10ти юзеров письма, то это можно сделать за 5 вызовов GetMessages(). в тестах проверяется, что кол-во вызовов оптимальное


CheckSpam()

in - MsgID внутри interface{}. это айдишники писем
out - MsgData{}. это структура с парой полей: id и факт того является ли письмо спамом. это результат работы HasSpam()
особенности:
HasSpam() симулирует поход в сервис антиспама, чтоб проверить письмо на наличие спама. один запрос выполняется за 100мс. и у этого сервиса есть "антибрут" - его нельзя вызывать бесконтрольно в кучу потоков. если сделать к нему более 5 параллельных запросов, то он начнет возвращать ошибку и данные о наличии спама вы не получите.

CombineResults()

in - MsgData внутри interface{}
out - string. это строки вида "<has_spam> <msg_id>", например "true 17696166526272393238"
особенности:
CombineResults() ждет все результаты из in, а потом сортирует их по наличию спама и по msg_id. то есть пример вывода может быть такой:

  true 123
  true 5555
  true 5556
  false 140
  false 3000
  false 3005

В чем подвох:
из-за описанных выше особенностей у вас либо не будет асинхрона и функции последовательно будут выполняться слишком долго
либо вы можете наоборот безконтрольно все распаралелить и тогда будут ошибки тк нарветесь на антибрут
либо вы не соптимизируете запросы "батчами" и будете лишний раз вызывать функции
на все расчеты у нас 3 сек. вообще суммарно по всем слипам должно быть 2.9сек, но в тестах округлил до 3секунд, чтоб сгладить погрешности рандома и самих вычислений. при этом обращаю внимание, что абсолютно верный код при запуске на винде без wsl может работать и немного дольше 3сек и тесты будут падать.

Код писать в spammer.go.
Запускать как go test -v -race
пример лога, для теста TestTotal:

2023/03/13 19:20:06.796173 [GetUser() 1.000291925s] args:harry.dubois@mail.ru res:{2436524555453976083 harry.dubois@mail.ru}
2023/03/13 19:20:06.796176 [GetUser() 1.00015549s] args:k.kitsuragi@mail.ru res:{6411680653583780021 k.kitsuragi@mail.ru}
2023/03/13 19:20:06.796290 [GetUser() 1.000175561s] args:noname@mail.ru res:{14126455436348693091 noname@mail.ru}
2023/03/13 19:20:06.796363 [GetUser() 1.000260181s] args:d.vader@mail.ru res:{15459287427396367812 d.vader@mail.ru}
2023/03/13 19:20:06.796562 [GetUser() 1.000200445s] args:spiderman@mail.ru res:{13707254613635009050 peter.parker@mail.ru}
2023/03/13 19:20:06.796754 [GetUser() 1.000397077s] args:e.musk@mail.ru res:{14529939612954000739 e.musk@mail.ru}
2023/03/13 19:20:06.797002 [GetUser() 1.000191749s] args:red_prince@mail.ru res:{1193889969480132348 red_prince@mail.ru}
2023/03/13 19:20:06.797452 [GetUser() 1.000418092s] args:tomasangelo@mail.ru res:{6935840902946149476 tomasangelo@mail.ru}
2023/03/13 19:20:06.797619 [GetUser() 1.000290256s] args:bruce.wayne@mail.ru res:{12499983457589032104 bruce.wayne@mail.ru}
2023/03/13 19:20:06.797704 [GetUser() 1.000487749s] args:batman@mail.ru res:{12499983457589032104 bruce.wayne@mail.ru}
2023/03/13 19:20:07.796781 [GetMessages() 1.000280874s] args:[{ID:2436524555453976083 Email:harry.dubois@mail.ru} {ID:6411680653583780021 Email:k.kitsuragi@mail.ru}] res:[12728377754914798838 14498495926778052146 12975933273041759035 12386730660396758454 9323185346293974544 8065084208075053255 10523043777071802347 12792092352287413255 12556782602004681106 12026159364158506481] err:<nil>
2023/03/13 19:20:07.796887 [GetMessages() 1.00016127s] args:[{ID:14126455436348693091 Email:noname@mail.ru} {ID:15459287427396367812 Email:d.vader@mail.ru}] res:[6652443725402098015 221945221381252775 9656111811170476016 221962074543525747 7594744397141820297] err:<nil>
2023/03/13 19:20:07.797408 [GetMessages() 1.000220051s] args:[{ID:13707254613635009050 Email:peter.parker@mail.ru} {ID:14529939612954000739 Email:e.musk@mail.ru}] res:[2803967521226628027 13245035231559086127 10493933060383355848 378045830174189628 10462184946173556768 10167774218733491071 11204847394727393252 10463884548348336960 15784986543485231004 16476037061321929257 17259218828069106373 4652873815360231330 357347175551886490 11512743696420569029 1595319133252549342 17696166526272393238] err:<nil>
2023/03/13 19:20:07.798014 [GetMessages() 1.000146318s] args:[{ID:12499983457589032104 Email:bruce.wayne@mail.ru}] res:[14107154567229229487 15728889559763622673 15262116397886015961 59892029605752939 17087986564527251681] err:<nil>
2023/03/13 19:20:07.798147 [GetMessages() 1.000366665s] args:[{ID:1193889969480132348 Email:red_prince@mail.ru} {ID:6935840902946149476 Email:tomasangelo@mail.ru}] res:[1877225754447839300 5108368734614700369 16728486308265447483 7829088386935944034 26236336874602209 15161554273155698590] err:<nil>
2023/03/13 19:20:07.897411 [HasSpam() 100.466053ms] args:12975933273041759035 res:false err:<nil>
2023/03/13 19:20:07.897455 [HasSpam() 100.541133ms] args:12386730660396758454 res:true err:<nil>
2023/03/13 19:20:07.897522 [HasSpam() 100.630163ms] args:12728377754914798838 res:true err:<nil>
2023/03/13 19:20:07.897581 [HasSpam() 100.664033ms] args:14498495926778052146 res:false err:<nil>
2023/03/13 19:20:07.897619 [HasSpam() 100.706127ms] args:9323185346293974544 res:true err:<nil>
2023/03/13 19:20:07.998889 [HasSpam() 100.871153ms] args:2803967521226628027 res:false err:<nil>
2023/03/13 19:20:07.998986 [HasSpam() 101.16548ms] args:6652443725402098015 res:false err:<nil>
2023/03/13 19:20:07.999003 [HasSpam() 100.948616ms] args:14107154567229229487 res:true err:<nil>
2023/03/13 19:20:07.999109 [HasSpam() 100.779238ms] args:1877225754447839300 res:true err:<nil>
2023/03/13 19:20:07.999219 [HasSpam() 101.678372ms] args:8065084208075053255 res:true err:<nil>
2023/03/13 19:20:08.099965 [HasSpam() 100.407738ms] args:5108368734614700369 res:true err:<nil>
2023/03/13 19:20:08.100040 [HasSpam() 101.001936ms] args:10523043777071802347 res:false err:<nil>
2023/03/13 19:20:08.100106 [HasSpam() 100.748952ms] args:13245035231559086127 res:true err:<nil>
2023/03/13 19:20:08.100511 [HasSpam() 101.280179ms] args:221945221381252775 res:true err:<nil>
2023/03/13 19:20:08.100515 [HasSpam() 101.041195ms] args:15728889559763622673 res:false err:<nil>
2023/03/13 19:20:08.200512 [HasSpam() 100.385719ms] args:12792092352287413255 res:false err:<nil>
2023/03/13 19:20:08.200931 [HasSpam() 100.350373ms] args:9656111811170476016 res:false err:<nil>
2023/03/13 19:20:08.201100 [HasSpam() 100.074524ms] args:16728486308265447483 res:true err:<nil>
2023/03/13 19:20:08.201253 [HasSpam() 100.292945ms] args:10493933060383355848 res:false err:<nil>
2023/03/13 19:20:08.201424 [HasSpam() 100.347645ms] args:15262116397886015961 res:false err:<nil>
2023/03/13 19:20:08.301988 [HasSpam() 100.459913ms] args:7829088386935944034 res:true err:<nil>
2023/03/13 19:20:08.302033 [HasSpam() 101.361212ms] args:12556782602004681106 res:true err:<nil>
2023/03/13 19:20:08.302165 [HasSpam() 100.869441ms] args:59892029605752939 res:false err:<nil>
2023/03/13 19:20:08.302003 [HasSpam() 100.774148ms] args:378045830174189628 res:false err:<nil>
2023/03/13 19:20:08.302095 [HasSpam() 101.087032ms] args:221962074543525747 res:false err:<nil>
2023/03/13 19:20:08.403435 [HasSpam() 100.724433ms] args:17087986564527251681 res:true err:<nil>
2023/03/13 19:20:08.403508 [HasSpam() 100.866513ms] args:7594744397141820297 res:false err:<nil>
2023/03/13 19:20:08.403885 [HasSpam() 101.683938ms] args:12026159364158506481 res:true err:<nil>
2023/03/13 19:20:08.403841 [HasSpam() 101.186792ms] args:10462184946173556768 res:false err:<nil>
2023/03/13 19:20:08.404080 [HasSpam() 101.370904ms] args:26236336874602209 res:false err:<nil>
2023/03/13 19:20:08.504547 [HasSpam() 100.426661ms] args:11204847394727393252 res:true err:<nil>
2023/03/13 19:20:08.504585 [HasSpam() 100.322008ms] args:10463884548348336960 res:true err:<nil>
2023/03/13 19:20:08.504763 [HasSpam() 100.467656ms] args:15784986543485231004 res:false err:<nil>
2023/03/13 19:20:08.504866 [HasSpam() 101.19378ms] args:10167774218733491071 res:false err:<nil>
2023/03/13 19:20:08.505053 [HasSpam() 101.129515ms] args:15161554273155698590 res:false err:<nil>
2023/03/13 19:20:08.606136 [HasSpam() 101.056629ms] args:4652873815360231330 res:true err:<nil>
2023/03/13 19:20:08.606264 [HasSpam() 101.323123ms] args:17259218828069106373 res:true err:<nil>
2023/03/13 19:20:08.606340 [HasSpam() 101.611667ms] args:16476037061321929257 res:true err:<nil>
2023/03/13 19:20:08.606668 [HasSpam() 101.214324ms] args:11512743696420569029 res:false err:<nil>
2023/03/13 19:20:08.606759 [HasSpam() 101.583438ms] args:357347175551886490 res:true err:<nil>
2023/03/13 19:20:08.706942 [HasSpam() 100.410502ms] args:17696166526272393238 res:true err:<nil>
2023/03/13 19:20:08.706983 [HasSpam() 100.669007ms] args:1595319133252549342 res:true err:<nil>

Подсказки:

помните что порядок выполнения горутин заранее не предопредлен
задание построено так, чтобы хорошо разобраться со всем материалом лекции, т.е. вдумчиво посмотреть примеры и применить их на практике. искать по гуглу или стек оферфлоу ничего не надо
вам не надо накапливать данные - сразу передаем их дальше ( например awk из кода выше - на это есть отдельный тест. Разве что функция сама не решает накопить - у нас это CombineResults или sort из кода выше)
подумайте, как будет организовано завершение функции если данные конечны. Что для этого надо сделать?
если вам встретился рейс ( опция -race ) - исследуйте его вывод - когда читаем, когда пишем, из каких строк кода. там как правило содержится достаточно информации для нахождения источника проблемы.
прежде чем приступать к распараллеливанию функций, чтобы уложиться в отведенный таймаут - сначала напишите линейный код, который будет выдавать правильный результат, лучше даже начать с меньшего количества значений чтобы совпадало с тем что в задании
ответ на вопрос "когда закрывается цикл по каналу" помогает в реализации RunPipeline

Хорошо помогает нарисовать схему расчетов
естественно нельзя самим вычислять данные в обход предоставляемых функций - их вызов будет проверяться.
select в этом задании не нужен
time.Sleep использовать нельзя

Эталонное решение занимает 130 строк
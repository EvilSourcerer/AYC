var getrequests = new XMLHttpRequest();
getrequests.addEventListener("load", function () {
    var orders=JSON.parse(this.responseText);
    var recenttransactions=document.getElementById("recenttransactions");
    var echostring=`<h1 style="text-align: center;color: rgb(255,255,255);font-weight: normal;font-size: 19px;">Recent Transactions</h1>`;
    for(var i=0; i<orders.length; i++) {
        echostring+=`<div class="d-flex align-items-center" style="margin-top: 1%;height: 40px;width: 90%;margin-left: 5%;background-color: rgba(50,48,50,0.45);"> <h1 style="font-size: 21px;margin-left: 5%;font-weight: normal;font-style: normal;color: rgb(143,143,143);">` + escapeHTML(orders[i]["item_name"].toString()) + `</h1> <h1 style="font-size: 21px;margin-right: 5%;margin-left: auto;font-weight: normal;font-style: normal;color: rgb(0,170,27);">` + escapeHTML(orders[i]["price"].toString()) + ` R€</h1> </div>`;
    }
    recenttransactions.innerHTML=echostring;
});
getrequests.open("GET", "neworders");
getrequests.send();
setInterval(function () {
    getrequests.open("GET", "neworders");
    getrequests.send();
}, 5000);
function escapeHTML(unsafeText) {
    let div = document.createElement('div');
    div.innerText = unsafeText;
    return div.innerHTML;
}
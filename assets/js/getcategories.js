var getrequests = new XMLHttpRequest();
var selectedcategory = "";
getrequests.addEventListener("load", function() {
    var categories = JSON.parse(this.responseText);
    var echostring = `<div class="card" style="background-color: rgba(64,64,64,0);"><div class="card-group">`;
    for (var i = 0; i < categories.length; i++) {
        echostring += `<div class="card" style="margin-left: 3px;margin-right: 3px;background-color: #363636;border: none;border-radius: 0px;"> <div class="card-body" style="text-align: center;border: none;background-color: #363636;border-radius: 0px;"> <h4 class="card-title" style="text-align: center;color: rgb(187,187,187);">` + categories[i]["item_name"] + `</h4><img src="` + categories[i]["item_photo"] + `" style="width: 10%;" draggable="false"> </div> </div>`;
    }
    echostring += `</div> </div> </div> </div>`;
    document.getElementById("categoryecho").innerHTML = echostring;
    var categories = document.querySelectorAll(".card-body");
    for (var i = 0; i < categories.length; i++) {
        categories[i].addEventListener("click", function() {
            this.style.backgroundColor = "#444444";
            for (var i = 0; i < categories.length; i++) {
                if (categories[i] != this) {
                    categories[i].style.backgroundColor = "#363636";
                }
            }
            selectedcategory = this.firstChild.nextSibling.innerHTML;
        });
    }
});
var marketfetcher=new XMLHttpRequest();
marketfetcher.addEventListener("load",function() {
    var items=JSON.parse(this.responseText);
    console.log(items);
    var echostring=`<div class="d-flex align-items-center" style="padding-left: 2%;padding-top: 2%;border-radius: 0;"><button class="btn btn-primary" type="button" style="box-shadow: none;border-radius: 0px;background-color: rgba(255,255,255,0.19);border: 0;font-size: 16px;" data-toggle="modal" data-target="#item-filters" onclick="getCategories()"><i class="fas fa-sliders-h" style="font-size: 16px;"></i><span class="pull-right" style="margin-left: 5px;float: right;font-size: 16px;">Item filters...</span></button></div><div class="d-flex align-items-center my-item" style="width: 96%;margin-left: 2%;height: 60px;margin-top: 1%;background-color: rgba(62,62,62,0.66);">`;
    for(var i=0; i<items.length; i++) {
        var tempitem=JSON.parse(items[i]);
        echostring+=`<h1 style="margin-left: 1%;font-size: 23px;color: rgb(255,255,255);font-weight: normal;font-style: normal;margin-top: 5px;">` +tempitem["item_name"]+ `</h1> <h1 style="margin-left: auto;margin-right: 1%;font-size: 23px;color: rgb(255,255,255);font-weight: normal;font-style: normal;margin-top: 5px;">LEIJURV WHAT'S THE PRICE R€</h1>`;
    }
    echostring+=`</div>`;
    document.getElementById("tab-2").innerHTML=echostring;
});
document.getElementById("categoryselector").addEventListener("click",function() {
    getMarket(selectedcategory);
});

function getMarket(category) {
    marketfetcher.open("GET","market?category=" + category);
    marketfetcher.send();
}

function getCategories() {
    getrequests.open("GET", "categories");
    getrequests.send();
}

function escapeHTML(unsafeText) {
    let div = document.createElement('div');
    div.innerText = unsafeText;
    return div.innerHTML;
}
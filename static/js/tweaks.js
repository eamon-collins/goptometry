//Tweaks for the goptometry module by Eamon Collins

//map of competitors equivalent models
var model_map = {
  "Google": {
    "aaa03c23b3724a16a56b629203edc62c" : "General", //general
    "eee28c313d69466f836ab83287a54ed9" : "General", //Travel
    "bd367be194cf45149e75f01d59f77ba7" : "General", //Food
    "e9576d86d2004ed1a38ba0cf39ecb4b1" : "NSFW",    //NSFW
    "e466caa0619f444ab97497640cefc4dc" : "General",  //Celeb
    "e0be3b9d6a454f0493ac3a30784001ff" : "General",  //apparel
    "c0c0ac362b03416da06ab3fa36fb58e3" : "General", //demographics
    "c443119bf2ed4da98487520d01a0b1e3" : "Logo",    //Logo
    "a403429f2ddf4b49b307e318f00e528b" : "Face", //Face detect
  },
  "Microsoft": {
    "aaa03c23b3724a16a56b629203edc62c" : "General", //general
    "eee28c313d69466f836ab83287a54ed9" : "General", //Travel
    "bd367be194cf45149e75f01d59f77ba7" : "General", //Food
    "e9576d86d2004ed1a38ba0cf39ecb4b1" : "NSFW",    //NSFW
    "e466caa0619f444ab97497640cefc4dc" : "Celebrity",  //Celeb
    "e0be3b9d6a454f0493ac3a30784001ff" : "General",  //apparel
    "c0c0ac362b03416da06ab3fa36fb58e3" : "General", //demographics
    "c443119bf2ed4da98487520d01a0b1e3" : "General",    //Logo
    "a403429f2ddf4b49b307e318f00e528b" : "Face", //Face detect
  },
  "Amazon": {
    "aaa03c23b3724a16a56b629203edc62c" : "General", //general
    "eee28c313d69466f836ab83287a54ed9" : "General", //Travel
    "bd367be194cf45149e75f01d59f77ba7" : "General", //Food
    "e9576d86d2004ed1a38ba0cf39ecb4b1" : "NSFW",    //NSFW
    "e466caa0619f444ab97497640cefc4dc" : "Celebrity",  //Celeb
    "e0be3b9d6a454f0493ac3a30784001ff" : "General",  //apparel
    "c0c0ac362b03416da06ab3fa36fb58e3" : "General", //demographics
    "c443119bf2ed4da98487520d01a0b1e3" : "General",    //Logo
    "a403429f2ddf4b49b307e318f00e528b" : "Face", //Face detect
  },
  "IBM": {
    "aaa03c23b3724a16a56b629203edc62c" : "General", //general
    "eee28c313d69466f836ab83287a54ed9" : "General", //Travel
    "bd367be194cf45149e75f01d59f77ba7" : "General", //Food
    "e9576d86d2004ed1a38ba0cf39ecb4b1" : "General",    //NSFW
    "e466caa0619f444ab97497640cefc4dc" : "General",  //Celeb
    "e0be3b9d6a454f0493ac3a30784001ff" : "General",  //apparel
    "c0c0ac362b03416da06ab3fa36fb58e3" : "General", //demographics
    "c443119bf2ed4da98487520d01a0b1e3" : "General",    //Logo
    "a403429f2ddf4b49b307e318f00e528b" : "Face", //Face detect
  },
}

$(document).ready(function() {
  //takes a snapshot of the html of the response and writes it to file
  //used for an archive of previous requests
  if($("#archive-page").length > 0) {
    //construct the string of html to be written to archive
    var htmlString = `<div class="buffer"><div class="row"><div class="col-md-2">`
    htmlString += $("#analyzedImage")[0].outerHTML
    htmlString += `</div>`
    $(".response-col").each(function() {
      htmlString += $(this)[0].outerHTML
    })
    htmlString += `</div></div>`

    //ajax call back to go server to store the html in the archive
    $.ajax({url: 'http://localhost:8000/writearchive/', type: 'POST', data: htmlString})
  }
  //expand the clarifai response initially, except on the archive page
  //don't have it expanded by default so that when the archive stores it, it is
  //collapsed and compact
  if($("#currently-archive").length == 0) {
    $("#collapseClarifai").collapse('show')
  }

  //When a clarifai model is selected, change the text boxes for each competitor to reflect their equivalent model/function
  $("#model_id").change(function(){
    $("#competitor-fieldset .comp-model-text").each(function(){
      $(this).val(model_map[$(this).siblings("label").children(".comp-check").val()][$("#model_id").val()])
    })
  })

  //takes care of clickable area for accordion items
  $(".accordiontag").click(function(){
    $(this).siblings(".panel-collapse").collapse('toggle')
  })

  //expands clickable area of competitor checkbox to whole panel
  $(".comp-panel").click(function(){
    var check = $(this).find("input.comp-check")
    check.prop("checked", !check.prop("checked"))
  })

  $("#reset-archive").click(function(){
    //resets archive, POST so that you can't do it by simply visiting the url
    $.ajax({url: 'http://localhost:8000/resetarchive/', type: 'POST', data: "delete"})
  })

})

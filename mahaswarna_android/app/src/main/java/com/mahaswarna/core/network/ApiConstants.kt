package com.mahaswarna.core.network

object ApiConstants {

    const val API_VERSION = "v1"

    // BuildConfig values are set per build type in app/build.gradle.kts.
    // Debug  → http://10.0.2.2:4000/v1/   ws://10.0.2.2:4002
    // Staging → https://staging-api.mahaswarna.com/v1/
    // Release → https://api.mahaswarna.com/v1/

    // 61-city compile-time constant — NOT fetched from a backend endpoint.
    // If cities are added in future, a new app release is required (accepted v1 trade-off).
    // NOTE: PRD §3 states "61 cities"; the canonical backend list (backend-architecture.md) has 63.
    // Using the canonical backend list as source of truth — it matches what the DB seeds.
    val CITY_LIST: List<City> = listOf(
        City("mumbai",              "Mumbai"),
        City("delhi",               "Delhi"),
        City("kolkata",             "Kolkata"),
        City("bangalore",           "Bangalore"),
        City("hyderabad",           "Hyderabad"),
        City("chennai",             "Chennai"),
        City("ahmedabad",           "Ahmedabad"),
        City("pune",                "Pune"),
        City("jaipur",              "Jaipur"),
        City("surat",               "Surat"),
        City("lucknow",             "Lucknow"),
        City("kanpur",              "Kanpur"),
        City("nagpur",              "Nagpur"),
        City("visakhapatnam",       "Visakhapatnam"),
        City("indore",              "Indore"),
        City("thane",               "Thane"),
        City("bhopal",              "Bhopal"),
        City("patna",               "Patna"),
        City("ludhiana",            "Ludhiana"),
        City("agra",                "Agra"),
        City("rajkot",              "Rajkot"),
        City("coimbatore",          "Coimbatore"),
        City("vadodara",            "Vadodara"),
        City("amritsar",            "Amritsar"),
        City("meerut",              "Meerut"),
        City("nashik",              "Nashik"),
        City("faridabad",           "Faridabad"),
        City("ghaziabad",           "Ghaziabad"),
        City("aurangabad",          "Aurangabad"),
        City("ranchi",              "Ranchi"),
        City("howrah",              "Howrah"),
        City("jodhpur",             "Jodhpur"),
        City("guwahati",            "Guwahati"),
        City("chandigarh",          "Chandigarh"),
        City("solapur",             "Solapur"),
        City("jabalpur",            "Jabalpur"),
        City("madurai",             "Madurai"),
        City("raipur",              "Raipur"),
        City("kota",                "Kota"),
        City("kalyan",              "Kalyan"),
        City("vasai-virar",         "Vasai-Virar"),
        City("allahabad",           "Allahabad"),
        City("vijayawada",          "Vijayawada"),
        City("srinagar",            "Srinagar"),
        City("amravati",            "Amravati"),
        City("navi-mumbai",         "Navi Mumbai"),
        City("pimpri-chinchwad",    "Pimpri-Chinchwad"),
        City("thiruvananthapuram",  "Thiruvananthapuram"),
        City("hubli",               "Hubli"),
        City("kochi",               "Kochi"),
        City("mangalore",           "Mangalore"),
        City("mysuru",              "Mysuru"),
        City("hubli-dharwad",       "Hubli-Dharwad"),
        City("warangal",            "Warangal"),
        City("guntur",              "Guntur"),
        City("nellore",             "Nellore"),
        City("tirunelveli",         "Tirunelveli"),
        City("bhiwandi",            "Bhiwandi"),
        City("saharanpur",          "Saharanpur"),
        City("gorakhpur",           "Gorakhpur"),
        City("bikaner",             "Bikaner"),
        City("noida",               "Noida"),
        City("gurgaon",             "Gurgaon"),
    )

    val DEFAULT_CITY: City = CITY_LIST.first { it.id == "mumbai" }

    data class City(val id: String, val displayName: String)
}

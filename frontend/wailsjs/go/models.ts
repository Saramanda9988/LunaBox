export namespace appconf {
	
	export class AppConfig {
	    access_token?: string;
	    vndb_access_token?: string;
	    theme: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.access_token = source["access_token"];
	        this.vndb_access_token = source["vndb_access_token"];
	        this.theme = source["theme"];
	        this.language = source["language"];
	    }
	}

}

export namespace enums {
	
	export enum SourceType {
	    LOCAL = "local",
	    BANGUMI = "bangumi",
	    VNDB = "vndb",
	}

}

export namespace models {
	
	export class Category {
	    id: string;
	    user_id: string;
	    name: string;
	    is_system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.user_id = source["user_id"];
	        this.name = source["name"];
	        this.is_system = source["is_system"];
	    }
	}
	export class Game {
	    id: string;
	    user_id: string;
	    name: string;
	    cover_url: string;
	    company: string;
	    summary: string;
	    path: string;
	    source_type: enums.SourceType;
	    // Go type: time
	    cached_at: any;
	    source_id: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.user_id = source["user_id"];
	        this.name = source["name"];
	        this.cover_url = source["cover_url"];
	        this.company = source["company"];
	        this.summary = source["summary"];
	        this.path = source["path"];
	        this.source_type = source["source_type"];
	        this.cached_at = this.convertValues(source["cached_at"], null);
	        this.source_id = source["source_id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class User {
	    id: string;
	    // Go type: time
	    created_at: any;
	    default_backup_target: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.default_backup_target = source["default_backup_target"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sql {
	
	export class DB {
	
	
	    static createFrom(source: any = {}) {
	        return new DB(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace vo {
	
	export class AISummaryRequest {
	    chat_ids: string[];
	
	    static createFrom(source: any = {}) {
	        return new AISummaryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chat_ids = source["chat_ids"];
	    }
	}
	export class DailyPlayTime {
	    date: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyPlayTime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.duration = source["duration"];
	    }
	}
	export class GameDetailStats {
	    total_play_time: number;
	    today_play_time: number;
	    recent_play_history: DailyPlayTime[];
	
	    static createFrom(source: any = {}) {
	        return new GameDetailStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_play_time = source["total_play_time"];
	        this.today_play_time = source["today_play_time"];
	        this.recent_play_history = this.convertValues(source["recent_play_history"], DailyPlayTime);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GamePlayCount {
	    game_id: string;
	    game_name: string;
	    play_count: number;
	
	    static createFrom(source: any = {}) {
	        return new GamePlayCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.play_count = source["play_count"];
	    }
	}
	export class GamePlayStats {
	    game_id: string;
	    game_name: string;
	    total_duration: number;
	
	    static createFrom(source: any = {}) {
	        return new GamePlayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.total_duration = source["total_duration"];
	    }
	}
	export class GlobalStats {
	    total_play_time: number;
	    weekly_play_time: number;
	    play_time_leaderboard: GamePlayStats[];
	    most_played_game: GamePlayCount;
	
	    static createFrom(source: any = {}) {
	        return new GlobalStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_play_time = source["total_play_time"];
	        this.weekly_play_time = source["weekly_play_time"];
	        this.play_time_leaderboard = this.convertValues(source["play_time_leaderboard"], GamePlayStats);
	        this.most_played_game = this.convertValues(source["most_played_game"], GamePlayCount);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HomePageData {
	    recent_games: models.Game[];
	    recently_added: models.Game[];
	    today_play_time_sec: number;
	    weekly_play_time_sec: number;
	
	    static createFrom(source: any = {}) {
	        return new HomePageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recent_games = this.convertValues(source["recent_games"], models.Game);
	        this.recently_added = this.convertValues(source["recently_added"], models.Game);
	        this.today_play_time_sec = source["today_play_time_sec"];
	        this.weekly_play_time_sec = source["weekly_play_time_sec"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetadataRequest {
	    source: enums.SourceType;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.id = source["id"];
	    }
	}

}

